package util

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bingoohuang/gg/pkg/iox"

	"github.com/valyala/fasthttp"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/gg/pkg/codec/b64"
	"github.com/cespare/xxhash/v2"
)

func ReadBody(rsp *http.Response) ([]byte, error) {
	var bodyReader io.Reader
	if rsp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(rsp.Body)
		if err != nil {
			log.Printf("E! gzip read failed: %v", err)
			return nil, err
		}
		bodyReader = reader
	} else {
		bodyReader = rsp.Body
	}

	defer iox.Close(rsp.Body)

	rspBody, err := io.ReadAll(bodyReader)
	if err != nil {
		log.Printf("E! reading response body failed: %v", err)
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

func TimeoutPost(target, contentType string, data []byte, timeout time.Duration, header map[string]string) (int, string, error) {
	req := fasthttp.AcquireRequest()
	rsp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(rsp)

	req.Header.SetMethod(http.MethodPost)
	req.Header.SetRequestURI(target)
	for k, v := range header {
		req.Header.Add(k, v)
	}
	req.Header.Set("Content-Type", contentType)
	req.SetBody(data)

	var err error
	if timeout > 0 {
		err = fasthttp.DoTimeout(req, rsp, timeout)
	} else {
		err = fasthttp.Do(req, rsp)
	}

	if err != nil {
		return 0, "", err
	}

	return rsp.StatusCode(), string(rsp.Body()), nil
}

func TimeoutGet(target string, timeout time.Duration, header map[string]string) (int, string, error) {
	req := fasthttp.AcquireRequest()
	rsp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(rsp)

	req.Header.SetMethod(http.MethodGet)
	req.Header.SetRequestURI(target)
	for k, v := range header {
		req.Header.Add(k, v)
	}

	var err error
	if timeout > 0 {
		err = fasthttp.DoTimeout(req, rsp, timeout)
	} else {
		err = fasthttp.Do(req, rsp)
	}

	if err != nil {
		return 0, "", err
	}

	return rsp.StatusCode(), string(rsp.Body()), nil
}

func TimeoutInvoke(target, method string, header http.Header, body string, timeout time.Duration, headerFn func(k, v []byte)) (int, string, error) {
	req := fasthttp.AcquireRequest()
	rsp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(rsp)

	req.Header.SetMethod(method)
	req.Header.SetRequestURI(target)
	for k, v1 := range header {
		for _, v2 := range v1 {
			req.Header.Add(k, v2)
		}
	}
	req.SetBodyString(body)

	var err error
	if timeout > 0 {
		err = fasthttp.DoTimeout(req, rsp, timeout)
	} else {
		err = fasthttp.Do(req, rsp)
	}

	if err != nil {
		return 0, "", err
	}

	if headerFn != nil {
		rsp.Header.VisitAll(headerFn)
	}

	return rsp.StatusCode(), string(rsp.Body()), nil
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

func CopyStdHeader(dest, src http.Header, options ...CopyHeaderOptionFn) {
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

func CopyHeader(dest *fasthttp.RequestCtx, src http.Header, options ...CopyHeaderOptionFn) {
	option := &CopyHeaderOption{}
	for _, f := range options {
		f(option)
	}

	for k, vv := range src {
		if ss.AnyOf(k, option.Ignores...) {
			continue
		}

		for _, v := range vv {
			dest.Response.Header.Add(k, v)
		}
	}
}
