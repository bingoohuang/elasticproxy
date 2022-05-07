package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/bingoohuang/gg/pkg/ss"

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
		if util.MatchLabels(d, p.Labels) {
			p.Primaries = append(p.Primaries, d)
		}
	}

	if len(p.Primaries) == 0 {
		log.Fatalf("There is no primaries for elastic proxy :%d with labels %+v",
			p.ProxySource.Port, p.ProxySource.Labels)
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
	rw := &ResponseWriter{ResponseWriter: w}
	defer func() {
		if !rw.DataWritten {
			w.WriteHeader(accessLog.StatusCode)
		}
		accessLog.Duration = time.Now().Sub(startTime).String()
		log.Printf("access log: %s", codec.Json(accessLog))
	}()

	if r.Header.Get("Upgrade") == "websocket" {
		accessLog.StatusCode = http.StatusNotImplemented
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		accessLog.StatusCode = http.StatusInternalServerError
		return
	}

	rand.Shuffle(len(p.Primaries), func(i, j int) {
		p.Primaries[i], p.Primaries[j] = p.Primaries[j], p.Primaries[i]
	})

	for i, primary := range p.Primaries {
		p.invoke(rw, r, i == 0, primary, body, &accessLog)
	}
}

type ResponseWriter struct {
	http.ResponseWriter
	DataWritten bool
}

func (r *ResponseWriter) Write(data []byte) (int, error) {
	r.DataWritten = true
	return r.ResponseWriter.Write(data)
}

func (p *ElasticProxy) invoke(w http.ResponseWriter, r *http.Request, first bool, primary rest.Rest,
	body []byte, accessLog *model.AccessLog,
) {
	if err := model.RetryDo(p.ctx, func() error {
		p.invokeInternal(w, r, first, primary, body, accessLog)
		if util.InRange(accessLog.StatusCode, 200, 500) {
			return nil
		}
		return fmt.Errorf("bad status code: %d", accessLog.StatusCode)
	}); err != nil {
		log.Printf("retry failed: %v", err)
	}

	return
}

func (p *ElasticProxy) invokeInternal(w http.ResponseWriter, r *http.Request, first bool, primary rest.Rest,
	body []byte, accessLog *model.AccessLog,
) {
	target := util.JoinURL(primary.U, r.RequestURI)
	req, _ := http.NewRequest(r.Method, target, ioutil.NopCloser(bytes.NewBuffer(body)))
	req.Header = r.Header
	rsp, err := util.TimeoutInvoke(p.ctx, req, primary.Timeout)
	if err != nil {
		log.Printf("rest %s do failed: %v", target, err)
		return
	}

	if rsp.Body != nil {
		defer iox.Close(rsp.Body)
	}

	accessLog.StatusCode = rsp.StatusCode
	rspBody, _ := util.ReadBody(rsp)
	accessLog.ResponseBody = string(rspBody)

	if !first {
		log.Printf("rest %s status: %d, rsp body: %s", target, accessLog.StatusCode, rspBody)
		return
	}

	accessLog.Target = util.JoinURL(primary.U, r.RequestURI)

	if util.InRange(accessLog.StatusCode, 200, 300) &&
		!ss.AnyOf(r.Method, "GET", "HEAD") && p.ch != nil {
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
		if !ss.AnyOf(k, "Content-Length", "Content-Encoding") {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
	}

	w.WriteHeader(rsp.StatusCode)
	if _, err := w.Write(rspBody); err != nil {
		log.Printf("write data failed: %v", err)
	}
}
