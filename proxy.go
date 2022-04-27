package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bingoohuang/elasticproxy/backup/httpbackup"
	"github.com/bingoohuang/elasticproxy/backup/model"
	"github.com/bingoohuang/elasticproxy/backup/util"
	"github.com/bingoohuang/gg/pkg/ginx"
	"github.com/bingoohuang/gg/pkg/ss"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

// ElasticProxy forwards received HTTP calls to another HTTP server.
type ElasticProxy struct {
	primary        *url.URL
	backups        []string
	backupBeanChan chan model.BackupBean
}

// CreateElasticProxy returns a new elasticProxy object.
func CreateElasticProxy(elasticURL *url.URL, backups []string) *ElasticProxy {
	p := &ElasticProxy{
		primary: elasticURL,
		backups: backups,
	}

	if len(p.backups) > 0 {
		p.backupBeanChan = make(chan model.BackupBean)
		var beans []model.Backup
		for _, backup := range p.backups {
			if ss.HasPrefix(backup, "http:", "https:") {
				beans = append(beans, httpbackup.NewBackup(backup))
			}
		}

		go p.backupProcess(beans)
	}

	return p
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := 200
	target := util.JoinURL(p.primary, r.RequestURI)
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
	rsp, err := util.Client.Do(req)
	if err != nil {
		log.Printf("client do failed: %v", err)
		status = http.StatusInternalServerError
		return
	}

	status = rsp.StatusCode
	if status >= 200 && status < 300 && r.Method == http.MethodPost && p.backupBeanChan != nil {
		p.backupBeanChan <- model.BackupBean{Req: r, Body: body}
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

func (p *ElasticProxy) backupProcess(backups []model.Backup) {
	for bean := range p.backupBeanChan {
		for _, backup := range backups {
			backup.BackupOne(bean)
		}
	}
}

const ContentLengthKey = "Content-Length"
