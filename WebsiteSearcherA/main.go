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
	maxgo      = 20
	timeoutSec = 10
)

// HTTPResponse contains the http get response plus the relative url and err
type HTTPResponse struct {
	url      string
	response *http.Response
	err      error
}

// SearchResult contains the url and the result of the regex search
type SearchResult struct {
	url    string
	result bool
	err    error
}

func main() {
	start := time.Now()
	// parse command line arguments
	inPath := flag.String("inFile", "../urls.txt", "csv input file path")
	outPath := flag.String("outFile", "../results.txt", "the output file path")
	searchTerm := flag.String("searchTerm", "Treasure", "the search term")
	flag.Parse()

	// compile the search term into a regular expression
	regex, err := regexp.Compile(*searchTerm)
	check(err, "Cannot parse the regex")

	// reading urls from csv input file
	urlsc := readURLs(*inPath)
	// parallelFetch receives the urls from a string channel
	// and fetches the urls using a limited number of concurrent workers (maxgo)
	rsc := parallelFetch(urlsc)
	// parallelSearch receives from a HTTPResponse channel
	// check for err and search into the response body
	src := parallelSearch(rsc, regex)
	// writeResults receives from a SearchResult channel
	// and writes the data on the output file
	writeResults(*outPath, src)

	elapsed := time.Since(start)
	fmt.Printf("Website search took: %s\n", elapsed)
}

func readURLs(inPath string) <-chan string {
	urls := make(chan string)
	go func() {
		defer close(urls)
		// open input file
		inFile, err := os.Open(inPath)
		check(err, "Cannot open input file")
		defer inFile.Close()
		// parsing input file as a csv
		reader := csv.NewReader(inFile)
		// boolean to skip the headers
		headers := true
		for {
			line, err := reader.Read()
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
			urls <- line[1]
		}
	}()
	return urls
}

func parallelFetch(urlsc <-chan string) <-chan *HTTPResponse {
	rc := make(chan *HTTPResponse)
	client := getClient()
	var wg sync.WaitGroup
	wg.Add(maxgo)
	for i := 0; i < maxgo; i++ {
		go func(client *http.Client) {
			fetch(urlsc, rc, client)
			wg.Done()
		}(client)
	}
	go func() {
		wg.Wait()
		close(rc)
	}()
	return rc
}

func fetch(urlsc <-chan string, rc chan<- *HTTPResponse, client *http.Client) {
	for url := range urlsc {
		fmt.Printf("Fetching: %s\n", url)
		resp, err := client.Get("http://" + url)
		rc <- &HTTPResponse{url, resp, err}
		if err != nil && resp != nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
		}
	}
}

func parallelSearch(respc <-chan *HTTPResponse, regex *regexp.Regexp) <-chan *SearchResult {
	sr := make(chan *SearchResult)
	go func() {
		var wg sync.WaitGroup
		for r := range respc {
			wg.Add(1)
			go func(r *HTTPResponse) {
				search(r, sr, regex)
				wg.Done()
			}(r)
		}
		go func() {
			wg.Wait()
			close(sr)
		}()
	}()
	return sr
}

func search(resp *HTTPResponse, src chan<- *SearchResult, regex *regexp.Regexp) {
	fmt.Printf("Searching: %s\n", resp.url)
	if resp.err != nil {
		src <- &SearchResult{
			url:    resp.url,
			result: false,
			err:    resp.err,
		}
		return
	}
	bytes, err := ioutil.ReadAll(resp.response.Body)
	if err != nil {
		src <- &SearchResult{
			url:    resp.url,
			result: false,
			err:    err,
		}
		return
	}
	res := regex.Match(bytes)
	src <- &SearchResult{
		url:    resp.url,
		result: res,
		err:    nil,
	}
}

func writeResults(outPath string, src <-chan *SearchResult) {
	file, err := os.Create(outPath)
	check(err, "Cannot create output file")
	defer file.Close()

	writer := csv.NewWriter(file)

	writer.Write(createHeaders())
	for sr := range src {
		writer.Write(createRecord(sr))
	}

	writer.Flush()
	check(writer.Error(), "Cannot write into output file")
}

func createRecord(sr *SearchResult) []string {
	record := make([]string, 0, 3)
	record = append(record, fmt.Sprintf("%s", sr.url))
	record = append(record, fmt.Sprintf("%t", sr.result))
	if sr.err != nil {
		record = append(record, fmt.Sprintf("%s", sr.err))
	} else {
		record = append(record, "")
	}
	return record
}

func createHeaders() []string {
	record := make([]string, 0, 3)
	record = append(record, "url")
	record = append(record, "result")
	record = append(record, "error")
	return record
}

func getClient() *http.Client {
	timeout := time.Duration(timeoutSec * time.Second)
	client := &http.Client{
		Timeout: timeout,
	}
	return client
}

func check(err error, message string) {
	if err != nil {
		log.Fatalln(message, err)
	}
}
