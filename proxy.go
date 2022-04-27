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
	primary       *url.URL
	backups       []*url.URL
	backupReqChan chan backupReq
}

type backupReq struct {
	body []byte
	req  *http.Request
}

// CreateElasticProxy returns a new elasticProxy object.
func CreateElasticProxy(elasticURL *url.URL, backups []*url.URL) *ElasticProxy {
	p := &ElasticProxy{
		primary: elasticURL,
		backups: backups,
	}

	if len(p.backups) > 0 {
		p.backupReqChan = make(chan backupReq)
		go p.backupProcess()
	}

	return p
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := 200
	target := JoinURL(p.primary, r.RequestURI)
	fields := map[string]interface{}{
		"remote_addr": r.RemoteAddr,
		"method":      r.Method,
		"path":        r.URL.Path,
		"target":      target,
		"direction":   "primary",
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		fields["x_forwarded_for"] = xff
	}

	startTime := time.Now()
	headerWrote := false
	defer func() {
		fields["duration"] = time.Now().Sub(startTime).String()
		fields["status"] = status
		if !headerWrote {
			w.WriteHeader(status)
		}

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
		return
	}

	req, err := http.NewRequest(r.Method, target, ioutil.NopCloser(bytes.NewBuffer(body)))
	req.Header = r.Header
	rsp, err := client.Do(req)
	if err != nil {
		log.Printf("client do failed: %v", err)
		status = http.StatusInternalServerError
		return
	}

	status = rsp.StatusCode
	if status >= 200 && status < 300 && r.Method == http.MethodPost && p.backupReqChan != nil {
		p.backupReqChan <- backupReq{req: r, body: body}
	}

	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		log.Printf("reading response body failed: %v", err)
	}

	status = rsp.StatusCode
	for k, vv := range rsp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	if _, ok := rsp.Header[ContentLengthKey]; !ok && rsp.ContentLength > 0 {
		w.Header().Add(ContentLengthKey, fmt.Sprintf("%d", rsp.ContentLength))
	}

	w.WriteHeader(rsp.StatusCode)
	headerWrote = true
	if _, err := w.Write(data); err != nil {
		log.Printf("write data failed: %v", err)
	}
}

func (p *ElasticProxy) backupProcess() {
	for req := range p.backupReqChan {
		p.doBackup(req)
	}
}

func (p *ElasticProxy) doBackup(req backupReq) {
	for _, backup := range p.backups {
		p.doBackupOne(req, backup)
	}
}

func (p *ElasticProxy) doBackupOne(b backupReq, backup *url.URL) {
	status := 0
	target := JoinURL(backup, b.req.RequestURI)
	fields := map[string]interface{}{
		"direction": "backup",
		"target":    target,
	}
	startTime := time.Now()

	defer func() {
		fields["duration"] = time.Now().Sub(startTime).String()
		fields["status"] = status

		accessLog, _ := ginx.JsoniConfig.MarshalToString(context.Background(), fields)
		log.Print(accessLog)
	}()

	req, err := http.NewRequest(b.req.Method, target, ioutil.NopCloser(bytes.NewBuffer(b.body)))
	req.Header = b.req.Header
	rsp, err := client.Do(req)
	if err != nil {
		log.Printf("client do failed: %v", err)
		return
	}
	status = rsp.StatusCode
	if rsp.Body != nil {
		_, _ = io.Copy(ioutil.Discard, rsp.Body)
	}
}

const ContentLengthKey = "Content-Length"

var client = &http.Client{}

func JoinURL(base *url.URL, requestURI string) string {
	targetURL := *base
	targetURL.Path = path.Join(targetURL.Path, requestURI)
	return targetURL.String()
}
