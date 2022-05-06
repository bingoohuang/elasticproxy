package util

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
)

func ReadBody(rsp *http.Response) ([]byte, error) {
	var bodyReader io.Reader
	if rsp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(rsp.Body)
		if err != nil {
			log.Printf("gzip read failed: %v", err)
			return nil, err
		}
		bodyReader = reader
	} else {
		bodyReader = rsp.Body
	}

	rspBody, err := io.ReadAll(bodyReader)
	if err != nil {
		log.Printf("reading response body failed: %v", err)
	}
	return rspBody, err
}
