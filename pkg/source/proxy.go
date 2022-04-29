package source

import (
	"bytes"
	"context"
	"fmt"
	"github.com/bingoohuang/gg/pkg/iox"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/rest"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/ginx"
)

// ElasticProxy forwards received HTTP calls to another HTTP server.
type ElasticProxy struct {
	model.ProxySource

	server    *http.Server
	ctx       context.Context
	ch        chan<- model.Bean
	done      chan struct{}
	Primaries []rest.Rest
}

func (p *ElasticProxy) StartRead(ctx context.Context, primaries []rest.Rest, ch chan<- model.Bean) {
	addr := fmt.Sprintf(":%d", p.Port)
	p.server = &http.Server{Addr: addr, Handler: p}
	p.ctx = ctx
	p.ch = ch
	p.done = make(chan struct{})
	for _, d := range primaries {
		if d.MatchLabels(p.Labels) {
			p.Primaries = append(p.Primaries, d)
		}
	}
	if err := p.server.ListenAndServe(); err != nil {
		log.Printf("ListenAndServe failed: %v", err)
	}
	p.done <- struct{}{}
}

func (p *ElasticProxy) StopWait() {
	if err := p.server.Shutdown(p.ctx); err != nil {
		log.Printf("shutdown failed: %v", err)
	}
	<-p.done
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := 200
	fields := map[string]interface{}{
		"remote_addr": r.RemoteAddr,
		"method":      r.Method,
		"path":        r.URL.Path,
		"target":      r.RequestURI,
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

	first := true
	for _, primary := range p.Primaries {
		if !primary.MatchLabels(p.Labels) {
			continue
		}

		code, wrote := p.writePrimary(w, r, first, primary, body)
		if wrote {
			headerWrote = true
		}
		if first {
			status = code
			first = false
		}
	}
}

func (p *ElasticProxy) writePrimary(w http.ResponseWriter, r *http.Request, first bool, primary rest.Rest, body []byte,
) (status int, dataWrote bool) {
	target := util.JoinURL(primary.U, r.RequestURI)
	req, err := http.NewRequest(r.Method, target, ioutil.NopCloser(bytes.NewBuffer(body)))
	req.Header = r.Header
	rsp, err := util.Client.Do(req)
	if err != nil {
		log.Printf("rest %s do failed: %v", target, err)
		return http.StatusInternalServerError, false
	}

	log.Printf("rest %s do status: %d", target, rsp.StatusCode)

	if rsp.Body != nil {
		defer iox.Close(rsp.Body)
	}

	status = rsp.StatusCode
	var data []byte
	if first {
		if data, err = io.ReadAll(rsp.Body); err != nil {
			log.Printf("reading response body failed: %v", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, rsp.Body)
		return status, false
	}

	if status >= 200 && status < 300 && r.Method == http.MethodPost && p.ch != nil {
		rb := model.Bean{
			Host:       r.Host,
			RemoteAddr: r.RemoteAddr,
			Method:     r.Method,
			RequestURI: r.RequestURI,
			Labels:     p.Labels,
			Body:       body,
			Header:     http.Header{},
			ClusterIds: []string{primary.ClusterID},
		}
		rb.Header.Set("Content-Type", req.Header.Get("Content-Type"))

		p.ch <- rb
	}

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
	if _, err := w.Write(data); err != nil {
		log.Printf("write data failed: %v", err)
	}

	return status, true
}
