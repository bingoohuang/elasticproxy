package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/bingoohuang/gg/pkg/codec"
	"github.com/bingoohuang/gg/pkg/iox"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/rest"
	"github.com/bingoohuang/elasticproxy/pkg/util"
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
	accessLog := model.AccessLog{
		RemoteAddr: r.RemoteAddr,
		Method:     r.Method,
		Path:       r.URL.Path,
		Direction:  "primary",
	}

	accessLog.XForwardedFor = r.Header.Get("X-Forwarded-For")

	startTime := time.Now()
	headerWrote := false
	status := 200
	defer func() {
		if !headerWrote {
			w.WriteHeader(status)
		}
		accessLog.Duration = time.Now().Sub(startTime).String()
		accessLog.StatusCode = status

		log.Printf("access log: %s", codec.Json(accessLog))
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

		code, wrote := p.write1(w, r, first, primary, body, &accessLog)
		if wrote {
			headerWrote = true
		}
		if first {
			status = code
			first = false
			accessLog.Target = util.JoinURL(primary.U, r.RequestURI)
		}
	}
}

func (p *ElasticProxy) write1(w http.ResponseWriter, r *http.Request, first bool, primary rest.Rest, body []byte, accessLog *model.AccessLog) (status int, dataWrote bool) {
	if err := model.RetryWrite(p.ctx, func() error {
		status, dataWrote = p.write2(w, r, first, primary, body, accessLog)
		if status >= 200 && status < 300 {
			return nil
		}
		return fmt.Errorf("bad status code: %d", status)
	}); err != nil {
		log.Printf("retry failed: %v", err)
	}

	return
}

func (p *ElasticProxy) write2(w http.ResponseWriter, r *http.Request, first bool, primary rest.Rest, body []byte, accessLog *model.AccessLog) (status int, dataWrote bool) {
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
	rspBody, err := util.ReadBody(rsp)
	if err != nil {
		return status, false
	}
	accessLog.ResponseBody = string(rspBody)

	log.Printf("rsp status: %d, rsp body: %s", status, rspBody)

	if !first {
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
			Body:       string(body),
			Header:     http.Header{},
			ClusterIds: []string{primary.ClusterID},
		}
		rb.Header.Set("Content-Type", req.Header.Get("Content-Type"))

		p.ch <- rb
	}

	for k, vv := range rsp.Header {
		if k == "Content-Length" {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(rsp.StatusCode)
	if _, err := w.Write(rspBody); err != nil {
		log.Printf("write data failed: %v", err)
	}

	return status, true
}
