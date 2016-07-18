package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
)

var (
	key    = flag.String("key", "treqs", "tracer key")
	method = flag.String("method", "GET", "HTTP request method")
	url    = flag.String("url", "", "request URL")
)

func main() {
	flag.Parse()

	if *url == "" {
		log.Fatal("missing -url argument")
	}

	req, err := http.NewRequest(*method, *url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-Treqs-Action", "trace")
	req.Header.Set("X-Treqs-Key", *key)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	traceID := res.Header.Get("X-Treqs-Id")
	if traceID == "" {
		log.Fatal("trace request failed, X-Treqs-Id header missing in response")
	}

	if req, err = http.NewRequest(*method, *url, nil); err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-Treqs-Action", "read")
	req.Header.Set("X-Treqs-Id", traceID)
	req.Header.Set("X-Treqs-Key", *key)

	if res, err = http.DefaultClient.Do(req); err != nil {
		log.Fatal(err)
	}
	if _, err = io.Copy(os.Stdout, res.Body); err != nil {
		log.Fatal(err)
	}
}
