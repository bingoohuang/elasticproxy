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
	"strconv"
	"time"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/rest"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/codec"
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
		log.Printf("E! ListenAndServe failed: %v", err)
	}
	p.done <- struct{}{}
}

func (p *ElasticProxy) StopWait() {
	if err := p.server.Shutdown(p.ctx); err != nil {
		log.Printf("E! shutdown failed: %v", err)
	}
	<-p.done
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.checkHeader(w, r) {
		return
	}

	accessLog := model.AccessLog{
		RemoteAddr: r.RemoteAddr,
		Method:     r.Method,
		Path:       r.URL.Path,
		Direction:  "primary",
		StatusCode: http.StatusBadGateway,
	}

	accessLog.XForwardedFor = w.Header().Get("X-Forwarded-For")

	startTime := time.Now()
	rw := &ResponseWriter{ResponseWriter: w}
	defer func() {
		if !rw.statusCodeWritten {
			w.WriteHeader(accessLog.StatusCode)
		}

		if !p.NoAccessLog {
			accessLog.Duration = time.Now().Sub(startTime).String()
			log.Printf("access log: %s", codec.Json(accessLog))
		}
	}()

	if w.Header().Get("Upgrade") == "websocket" {
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

func (p *ElasticProxy) checkHeader(w http.ResponseWriter, r *http.Request) bool {
	header := r.Header
	for k, v := range p.Header {
		if header.Get(k) == v {
			continue
		}

		w.WriteHeader(http.StatusUnauthorized)
		switch {
		case k == "Authorization" && ss.HasPrefix(v, "Basic "):
			w.Header().Set("WWW-Authenticate", "Basic realm="+strconv.Quote("Authorization Required"))
		default:
		}

		return false
	}

	return true
}

type ResponseWriter struct {
	http.ResponseWriter
	statusCodeWritten bool
}

func (r *ResponseWriter) WriteHeader(statusCode int) {
	r.statusCodeWritten = true
	r.ResponseWriter.WriteHeader(statusCode)
}

func (p *ElasticProxy) invoke(w http.ResponseWriter, r *http.Request, first bool, pr rest.Rest,
	body []byte, accessLog *model.AccessLog,
) {
	if err := model.RetryDo(p.ctx, func() error {
		p.invokeInternal(w, r, first, pr, body, accessLog)
		if util.InRange(accessLog.StatusCode, 200, 500) {
			return nil
		}
		return fmt.Errorf("bad status code: %d, response: %s", accessLog.StatusCode, accessLog.ResponseBody)
	}); err != nil {
		log.Printf("E! retry failed: %v", err)
	}

	return
}

func (p *ElasticProxy) invokeInternal(w http.ResponseWriter, r *http.Request, first bool, pr rest.Rest,
	body []byte, accessLog *model.AccessLog,
) {
	target := util.JoinURL(pr.U, r.RequestURI)
	accessLog.Target = target
	req, _ := http.NewRequestWithContext(p.ctx, r.Method, target, ioutil.NopCloser(bytes.NewBuffer(body)))
	req.Header = r.Header
	for k, v := range pr.Header {
		req.Header.Add(k, v)
	}
	rsp, err := util.TimeoutInvoke(req, pr.Timeout)
	if err != nil {
		log.Printf("E! rest %s do failed: %v", target, err)
		return
	}

	accessLog.StatusCode = rsp.StatusCode
	rspBody, _ := util.ReadBody(rsp)
	accessLog.ResponseBody = string(rspBody)

	if !first {
		log.Printf("rest %s status: %d, rsp body: %s", target, accessLog.StatusCode, rspBody)
		return
	}

	if !util.InRange(accessLog.StatusCode, 200, 300) {
		return
	}

	if !ss.AnyOf(r.Method, "GET", "HEAD") && p.ch != nil {
		rb := model.Bean{
			Host:       r.Host,
			RemoteAddr: r.RemoteAddr,
			Method:     r.Method,
			RequestURI: r.RequestURI,
			Labels:     p.Labels,
			Body:       string(body),
			Header:     http.Header{},
			ClusterIds: []string{pr.ClusterID},
		}
		rb.Header.Set("Content-Type", req.Header.Get("Content-Type"))
		p.ch <- rb
	}

	w.WriteHeader(rsp.StatusCode)
	util.CopyHeader(w.Header(), rsp.Header, util.WithIgnores("Content-Length", "Content-Encoding"))
	if _, err := w.Write(rspBody); err != nil {
		log.Printf("E! write data failed: %v", err)
	}
}
