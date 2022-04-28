package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/bingoohuang/elasticproxy/pkg/backup/kaf"
	"github.com/bingoohuang/elasticproxy/pkg/backup/rest"
	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/ginx"
)

// ElasticProxy forwards received HTTP calls to another HTTP server.
type ElasticProxy struct {
	primary    *rest.Rest
	backupChan chan model.BackupBean
}

// CreateElasticProxy returns a new elasticProxy object.
func CreateElasticProxy(ctx context.Context, primary *rest.Rest, config *model.Config) *ElasticProxy {
	p := &ElasticProxy{
		primary:    primary,
		backupChan: createBackups(ctx, config),
	}

	return p
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := 200
	target := util.JoinURL(p.primary.U, r.RequestURI)
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
	if status >= 200 && status < 300 && r.Method == http.MethodPost && p.backupChan != nil {
		p.backupChan <- model.BackupBean{Req: r, Body: body}
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

	const k = "Content-Length"
	if _, ok := rsp.Header[k]; !ok && rsp.ContentLength > 0 {
		w.Header().Add(k, fmt.Sprintf("%d", rsp.ContentLength))
	}

	w.WriteHeader(rsp.StatusCode)
	headerWrote = true
	if _, err := w.Write(data); err != nil {
		log.Printf("write data failed: %v", err)
	}
}

func createBackups(ctx context.Context, c *model.Config) chan model.BackupBean {
	var beans []model.BackupWriter

	for _, backup := range c.Backups {
		if backup.Disabled {
			continue
		}
		restBackup := &rest.Rest{Elastic: backup}
		if err := restBackup.Initialize(); err != nil {
			log.Fatalf("initialize elastic backup failed: %v", err)
		}
		beans = append(beans, restBackup)
	}

	for _, k := range c.Kafkas {
		kafkaBackup := &kaf.Kafka{Kafka: k}
		if err := kafkaBackup.Initialize(); err != nil {
			log.Fatalf("initialize elastic backup failed: %v", err)
		}
		beans = append(beans, kafkaBackup)
	}

	if len(beans) > 0 {
		ch := make(chan model.BackupBean)
		go backupProcess(ctx, beans, ch)
		return ch
	}

	return nil
}

func backupProcess(ctx context.Context, backups []model.BackupWriter, ch chan model.BackupBean) {
	for bean := range ch {
		for _, backup := range backups {
			start := time.Now()
			err := backup.Write(ctx, bean)
			cost := time.Since(start)
			if err != nil {
				log.Printf("E! backup %s cost %s failed: %v", backup.Name(), cost, err)
			} else {
				log.Printf("backup %s cost %s successfully", backup.Name(), cost)
			}
		}
	}
}
