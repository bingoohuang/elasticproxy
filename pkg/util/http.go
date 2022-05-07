package util

import (
	"compress/gzip"
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bingoohuang/gg/pkg/codec/b64"
	"github.com/cespare/xxhash/v2"
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

func HashURL(u *url.URL) string {
	return Hash(u.String())
}

func Hash(s ...string) string {
	xx := xxhash.New()
	for _, i := range s {
		_, _ = xx.Write([]byte(i))
	}
	h, _ := b64.EncodeString(string(xx.Sum(nil)), b64.URL, b64.Raw)
	return h
}

func TimeoutInvoke(ctx context.Context, req *http.Request, timeout time.Duration) (*http.Response, error) {
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	return Client.Do(req)
}
