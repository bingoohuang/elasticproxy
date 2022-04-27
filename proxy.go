package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bingoohuang/gg/pkg/ginx"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"
)

// ElasticProxy forwards received HTTP calls to another HTTP server.
type ElasticProxy struct {
	primary *url.URL
	backups []*url.URL
}

// CreateElasticProxy returns a new elasticProxy object.
func CreateElasticProxy(elasticURL *url.URL, backups []*url.URL) *ElasticProxy {
	return &ElasticProxy{
		primary: elasticURL,
		backups: backups,
	}
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := 0
	fields := map[string]interface{}{
		"remote_addr": r.RemoteAddr,
		"method":      r.Method,
		"path":        r.URL.Path,
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		fields["x_forwarded_for"] = xff
	}

	startTime := time.Now()

	defer func() {
		fields["duration"] = time.Now().Sub(startTime).String()
		fields["status"] = status

		accessLog, _ := ginx.JsoniConfig.MarshalToString(context.Background(), fields)
		log.Print(accessLog)
	}()

	if r.Header.Get("Upgrade") == "websocket" {
		status = http.StatusNotImplemented
		w.WriteHeader(status)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		status = http.StatusInternalServerError
		w.WriteHeader(status)
		return
	}

	targetURL := *p.primary
	targetURL.Path = path.Join(targetURL.Path, r.RequestURI)
	targetURLAddr := targetURL.String()

	req, err := http.NewRequest(r.Method, targetURLAddr, ioutil.NopCloser(bytes.NewBuffer(body)))
	req.Header = r.Header
	rsp, err := client.Do(req)

	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		log.Printf("reading response body failed: %v", err)
	}
	for k, vv := range rsp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	if _, ok := rsp.Header[ContentLengthKey]; !ok && rsp.ContentLength > 0 {
		w.Header().Add(ContentLengthKey, fmt.Sprintf("%d", rsp.ContentLength))
	}

	status = rsp.StatusCode
	w.WriteHeader(rsp.StatusCode)
	if _, err := w.Write(data); err != nil {
		log.Printf("write data failed: %v", err)
	}
}

const ContentLengthKey = "Content-Length"

var client = &http.Client{}
