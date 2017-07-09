package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

const (
	timeoutSec = 10
	maxGo      = 20
)

func main() {
	start := time.Now()
	// parse command line arguments
	inFile := flag.String("inFile", "../urls.txt", "csv input file path")
	outFile := flag.String("outFile", "../results.txt", "the output file path")
	regEx := flag.String("regex", "Treasure", "the regular expression")
	flag.Parse()

	// open input file
	in, err := os.Open(*inFile)
	check(err)
	defer in.Close()

	// open output file
	out, err := os.Create(*outFile)
	check(err)
	defer out.Close()

	re, err := regexp.Compile(*regEx)
	check(err)

	// parsing input file as a csv
	r := csv.NewReader(in)

	// a buffered channel that works as a semaphore
	// only "maxGo" go routines at the same time
	sem := make(chan bool, maxGo)

	// boolean to skip the headers
	headers := true

	// build a http client with timeoutSec timeout
	client := getClient()

	// reading from the input file line by line and launch a go routine if an empty slot is available
	for {
		line, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalln(err)
		}
		if headers {
			headers = false
			continue
		}
		// booking a slot on the seamphore
		// if it's not available the program waits
		sem <- true
		go fetchAndSearch(line[1], re, client, out, sem)
	}

	// the programm will wait untill all the slots in the semaphore will be freed
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}
	elapsed := time.Since(start)
	fmt.Printf("Website search took: %s\n", elapsed)
}

// fetchAndSearch fetch the url and search a term using a regular expression
// the results of each url are written in the f file (csv format):
// true				-> search term found
// false			-> search term not found
// Error			-> error during the http request or during the reading of the response body
func fetchAndSearch(url string, re *regexp.Regexp, client *http.Client, f *os.File, sem chan bool) {
	// before returning, the semaphore slot is freed.
	defer func() { <-sem }()
	resp, err := client.Get("http://" + url)
	if err != nil {
		writeToFile(f, url, false, err)
		return
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		writeToFile(f, url, false, err)
		return
	}
	res := re.Match(bytes)
	writeToFile(f, url, res, nil)
}

var mu sync.Mutex

func writeToFile(f *os.File, url string, res bool, err error) {
	mu.Lock()
	defer mu.Unlock()
	s := ""
	if err != nil {
		s = fmt.Sprintf("%q,%t,%q\n", url, res, err)
	} else {
		s = fmt.Sprintf("%q,%t,\n", url, res)
	}
	f.WriteString(s)
}

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func getClient() *http.Client {
	timeout := time.Duration(timeoutSec * time.Second)
	client := &http.Client{
		Timeout: timeout,
	}
	return client
}
