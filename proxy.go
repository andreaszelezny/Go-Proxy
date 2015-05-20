// proxy.go
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const loggen = true

var (
	hopHeaders = []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te", // canonicalized version of "TE"
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	blackList        map[string]bool
	filterImageSizes bool
)

func main() {
	readFiles()
	http.ListenAndServe(":9000", http.HandlerFunc(myHandlerFunc))
}

func myHandlerFunc(rw http.ResponseWriter, req *http.Request) {
	/*
		if req.Method == "CONNECT" {
			log.Println("CONNECT: " + req.URL.Host)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
	*/
	if blackList[req.URL.Host] {
		log.Println("BLOCKED: " + req.RequestURI)
		//rw.WriteHeader(http.StatusGone) //StatusForbidden)
		//return
	} else {
		log.Println(req.RequestURI)
	}

	outreq := new(http.Request)
	*outreq = *req
	outreq.Proto = "HTTP/1.1"
	outreq.ProtoMajor = 1
	outreq.ProtoMinor = 1
	//outreq.Close = false

	// Copy header
	outreq.Header = make(http.Header)
	for i, hh := range req.Header {
		for _, h := range hh {
			outreq.Header.Add(i, h)
		}
	}
	for _, h := range hopHeaders {
		outreq.Header.Del(h)
		/*
			if outreq.Header.Get(h) != "" {
				outreq.Header.Del(h)
			}
		*/
	}
	//outreq.Header.Set("X-Forwarded-For", clientIP)	// random values for privacy?

	res, err := http.DefaultTransport.RoundTrip(outreq)
	if err != nil {
		log.Printf("Proxy error: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	for _, h := range hopHeaders {
		res.Header.Del(h)
	}

	var contentManipulated bool
	var bodyStr string
	if strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") && res.Header.Get("Content-Encoding") == "gzip" {
		res.Header.Del("Content-Encoding")
		res.Header.Del("Content-Length")
		if gzReader, zerr := gzip.NewReader(res.Body); zerr == nil {
			filterHtmlPage(gzReader)
			if body, zerr := ioutil.ReadAll(gzReader); zerr == nil {
				//filterHtmlPage(&body)
				bodyStr = string(body)
				contentManipulated = true
				log.Print(body[0:100])
			}
		}
	}

	// Copy header
	for i, hh := range res.Header {
		for _, h := range hh {
			rw.Header().Add(i, h)
		}
	}
	if contentManipulated {
		rw.Header().Add("Content-Length", strconv.Itoa(len(bodyStr)))
	}
	rw.WriteHeader(res.StatusCode)

	if !contentManipulated {
		io.Copy(rw, res.Body)
	} else {
		fmt.Fprint(rw, bodyStr)
	}

	/*
		client := &http.Client{}
		r.RequestURI = ""
		r.URL.Scheme = strings.Map(unicode.ToLower, r.URL.Scheme)

		resp, err := client.Do(r)
		if err != nil {
			log.Fatal(err)
		}
		resp.Write(w)
	*/
}

func filterHtmlPage(bodyReader io.Reader) ([]byte, error) {
	buffer := make([]byte)
	var bin bytes.Buffer
	var bout bytes.Buffer
	var btemp bytes.Buffer
	if _, err := bin.ReadFrom(bodyReader); err != nil {
		return nil, err
	}

}

func readFiles() {
	blackList = make(map[string]bool)

	file, err := os.Open("hosts.txt")
	if err != nil {
		log.Println("Error reading file")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var str string
	for scanner.Scan() {
		str = scanner.Text()
		if len(str) > 0 && str[0] != '#' {
			// todo: filter "localhost"
			blackList[strings.SplitN(str, " ", 3)[1]] = true
		}
	}
	if scanner.Err() != nil {
		log.Println("Error parsing file")
	}
}
