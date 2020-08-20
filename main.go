package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type probeArgs []string

func (p *probeArgs) Set(val string) error {
	*p = append(*p, val)
	return nil
}

func (p probeArgs) String() string {
	return strings.Join(p, ",")
}

func usage() {
	fmt.Println("Usage: cat tcp-ipp.csv | httprobe")
	fmt.Println("")
	fmt.Println("Get all HTTP(S) URLs based on input from ipp file. Outputs lines in format <proto>:<host>:<port>.")
	fmt.Println("See github.com/tomnomnom/httprobe for original script.")
	fmt.Println("")
	flag.PrintDefaults()
}

// Confirms HTTP is listening, doesn't care about virtual hosts
// Accepts <ipp.csv> file
func main() {

	flag.Usage = usage

	// concurrency flag
	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level (split equally between HTTPS and HTTP requests)")

	// timeout flag
	var to int
	flag.IntVar(&to, "t", 10000, "timeout (milliseconds)")

	// prefer https
	var preferHTTPS bool
	flag.BoolVar(&preferHTTPS, "prefer-https", false, "only try plain HTTP if HTTPS fails")

	// HTTP method to use
	var method string
	flag.StringVar(&method, "method", "GET", "HTTP method to use")

	flag.Parse()

	// make an actual time.Duration out of the timeout
	timeout := time.Duration(to * 1000000)

	var tr = &http.Transport{
		MaxIdleConns:      30,
		IdleConnTimeout:   time.Second,
		DisableKeepAlives: true,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: time.Second,
		}).DialContext,
	}

	re := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	client := &http.Client{
		Transport:     tr,
		CheckRedirect: re,
		Timeout:       timeout,
	}

	// domain/port pairs are initially sent on the httpsURLs channel.
	// If they are listening and the --prefer-https flag is set then
	// no HTTP check is performed; otherwise they're put onto the httpURLs
	// channel for an HTTP check.
	httpsURLs := make(chan string)
	httpURLs := make(chan string)
	output := make(chan string)

	// HTTPS workers
	var httpsWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpsWG.Add(1)

		go func() {
			for url := range httpsURLs {

				// always try HTTPS first
				withProto := "https://" + url
				if isListening(client, withProto, method) {
					output <- withProto

					// skip trying HTTP if --prefer-https is set
					if preferHTTPS {
						continue
					}
				}

				httpURLs <- url
			}

			httpsWG.Done()
		}()
	}

	// HTTP workers
	var httpWG sync.WaitGroup
	for i := 0; i < concurrency/2; i++ {
		httpWG.Add(1)

		go func() {
			for url := range httpURLs {
				withProto := "http://" + url
				if isListening(client, withProto, method) {
					output <- withProto
					continue
				}
			}

			httpWG.Done()
		}()
	}

	// Close the httpURLs channel when the HTTPS workers are done
	go func() {
		httpsWG.Wait()
		close(httpURLs)
	}()

	// Output worker
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go func() {
		for o := range output {
			fmt.Println(o)
		}
		outputWG.Done()
	}()

	// Close the output channel when the HTTP workers are done
	go func() {
		httpWG.Wait()
		close(output)
	}()

	// accept domains on stdin
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := strings.Split(strings.ToLower(sc.Text()), ",")
		host := line[0]

		// TODO add HTTP only option?

		for _, port := range line[1:] {
			// HTTP only is not supported now
			httpsURLs <- fmt.Sprintf("%s:%s", host, port)
		}
	}

	// once we've sent all the URLs off we can close the
	// input/httpsURLs channel. The workers will finish what they're
	// doing and then call 'Done' on the WaitGroup
	close(httpsURLs)

	// check there were no errors reading stdin (unlikely)
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

	// Wait until the output waitgroup is done
	outputWG.Wait()
}

func isListening(client *http.Client, url, method string) bool {

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return false
	}

	req.Header.Add("Connection", "close")
	req.Close = true

	resp, err := client.Do(req)
	if resp != nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	if err != nil {
		return false
	}

	return true
}
