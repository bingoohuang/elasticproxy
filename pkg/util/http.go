package util

import (
	"compress/gzip"
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bingoohuang/gg/pkg/ss"

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

func TimeoutPost(ctx context.Context, target, contentType string, body io.ReadCloser, timeout time.Duration, header map[string]string) (*http.Response, error) {
	req, _ := http.NewRequestWithContext(ctx, "POST", target, body)
	for k, v := range header {
		req.Header.Add(k, v)
	}
	req.Header.Set("Content-Type", contentType)

	return TimeoutInvoke(req, timeout)
}

func TimeoutGet(ctx context.Context, target string, timeout time.Duration, header map[string]string) (*http.Response, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", target, nil)
	for k, v := range header {
		req.Header.Add(k, v)
	}

	return TimeoutInvoke(req, timeout)
}

func TimeoutInvoke(req *http.Request, timeout time.Duration) (*http.Response, error) {
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	return Client.Do(req)
}

type CopyHeaderOption struct {
	Ignores []string
}

type CopyHeaderOptionFn func(*CopyHeaderOption)

func WithIgnores(headers ...string) CopyHeaderOptionFn {
	return func(o *CopyHeaderOption) {
		o.Ignores = append(o.Ignores, headers...)
	}
}

func CopyHeader(dest, src http.Header, options ...CopyHeaderOptionFn) {
	option := &CopyHeaderOption{}
	for _, f := range options {
		f(option)
	}

	for k, vv := range src {
		if ss.AnyOf(k, option.Ignores...) {
			continue
		}

		for _, v := range vv {
			dest.Add(k, v)
		}
	}
}
