# Website Searcher

Given a list of urls in urls.txt write a program that will fetch each page and determine whether a search term exists on the page (this search can be a really rudimentary regex - this part isn't too important).

You can make up the search terms. Ignore the additional information in the urls.txt file.

## Constraints

* Search is case insensitive.
* Should be concurrent.
* But! It shouldn't have more than 20 HTTP requests at any given time.
* The results should be writted out to a file results.txt
* The goal is to avoid using thread pooling libraries like ThreadPoolExecutor or Celluloid.

The solution can be written in any language, but Go or Java preferred.

## Solution

I created two possible solutions for this problem:

* solution A: it utilizes a pipeline to produce data and sends it downstream
* solution B: it utilizes a semaphore to limit the number of go routines

### Solution A
As mentioned before solution A utilizes a pipeline concept.
A pipeline is composed of stages connected by channels.
In every stage the go routines take values from inbound channels, perform operations and then send the results downstream into outbound channels.
The beauty of this concept is the modularity of each element of the pipeline. It's relatively easy to add more function into the pipeline to perform different opertions on the data.
In this particular case, I created a fixed number of workers to fetch the urls as requested.
##### Architecture
* readUrls returns a channel of strings and then sends into the channel the urls that it reads from the input file
* parallelFetch receives urls through an inbound channel that readUrls created. Then it uses an outbound channel to send downstream the http.Response obtained by a fixed number of concurrent workers.
* parallelSearch receives the http.Responses through an inbound channel that parallelFetch created. Then it searches (in a concurrent way) the search term into the response body and sends the results downstream into an outbound channel.
* writeResults receives the search results through an inbound channel that parallelSearch created and writes the output into a file.

### Solution B
Solutuon B uses a buffered channel as a semaphore to limit the number of go routines running at the same time. This solution lacks modularity and it is specifically tailored for this excercise. The reason why I am including this solution is because it runs faster then solution A even if it would be a problem to upgrade it.

##### Architecture
* buffered channel works as a semaphore. Only a limited number of slots are available.
* A new go routine can be processed only if a free slot is avalilable.
* If no slots are available, no new go routine will be launched.
* When a go routine finishes its job, it frees the solt.
