package source

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/rest"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/codec"
)

// ElasticProxy forwards received HTTP calls to another HTTP server.
type ElasticProxy struct {
	model.ProxySource

	server    *fasthttp.Server
	ctx       context.Context
	ch        chan<- model.Bean
	done      chan struct{}
	Primaries []rest.Rest
}

func (p *ElasticProxy) StartRead(ctx context.Context, primaries []rest.Rest, ch chan<- model.Bean) {
	p.server = &fasthttp.Server{Handler: p.ServeHTTP}
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

	addr := fmt.Sprintf(":%d", p.Port)
	// pass plain function to fasthttp
	if err := p.server.ListenAndServe(addr); err != nil {
		log.Printf("E! ListenAndServe failed: %v", err)
	}
	p.done <- struct{}{}
}

func (p *ElasticProxy) StopWait() {
	if err := p.server.Shutdown(); err != nil {
		log.Printf("E! shutdown failed: %v", err)
	}
	<-p.done
}

// ServeHTTP only forwards allowed requests to the real ElasticSearch server.
func (p *ElasticProxy) ServeHTTP(ctx *fasthttp.RequestCtx) {
	if !p.checkHeader(ctx) {
		return
	}

	accessLog := model.AccessLog{
		RemoteAddr: ctx.RemoteAddr().String(),
		Method:     string(ctx.Method()),
		Path:       string(ctx.Path()),
		Direction:  "primary",
		StatusCode: http.StatusBadGateway,
	}

	accessLog.XForwardedFor = string(ctx.Request.Header.Peek("X-Forwarded-For"))

	startTime := time.Now()
	rw := &ResponseWriter{RequestCtx: ctx}
	defer func() {
		if !rw.statusCodeWritten {
			ctx.SetStatusCode(accessLog.StatusCode)
		}

		if !p.NoAccessLog {
			accessLog.Duration = time.Now().Sub(startTime).String()
			log.Printf("access log: %s", codec.Json(accessLog))
		}
	}()

	if string(ctx.Request.Header.Peek("Upgrade")) == "websocket" {
		accessLog.StatusCode = http.StatusNotImplemented
		return
	}

	body := ctx.Request.Body()

	rand.Shuffle(len(p.Primaries), func(i, j int) {
		p.Primaries[i], p.Primaries[j] = p.Primaries[j], p.Primaries[i]
	})

	for i, primary := range p.Primaries {
		p.invoke(rw, i == 0, primary, body, &accessLog)
	}
}

func (p *ElasticProxy) checkHeader(ctx *fasthttp.RequestCtx) bool {
	for k, v := range p.Header {
		if string(ctx.Request.Header.Peek(k)) == v {
			continue
		}

		ctx.SetStatusCode(http.StatusUnauthorized)
		switch {
		case k == "Authorization" && ss.HasPrefix(v, "Basic "):
			ctx.Response.Header.Set("WWW-Authenticate", "Basic realm="+strconv.Quote("Authorization Required"))
		default:
		}

		return false
	}

	return true
}

type ResponseWriter struct {
	*fasthttp.RequestCtx
	statusCodeWritten bool
}

func (r *ResponseWriter) WriteHeader(statusCode int) {
	r.statusCodeWritten = true
	r.RequestCtx.SetStatusCode(statusCode)
}

func (p *ElasticProxy) invoke(rw *ResponseWriter, first bool, pr rest.Rest, body []byte, accessLog *model.AccessLog) {
	if err := model.RetryDo(p.ctx, func() error {
		p.invokeInternal(rw, first, pr, body, accessLog)
		if util.InRange(accessLog.StatusCode, 200, 500) {
			return nil
		}
		return fmt.Errorf("bad status code: %d, response: %s", accessLog.StatusCode, accessLog.ResponseBody)
	}); err != nil {
		log.Printf("E! retry failed: %v", err)
	}

	return
}

func (p *ElasticProxy) invokeInternal(rw *ResponseWriter, first bool, pr rest.Rest, body []byte, accessLog *model.AccessLog) {
	path := string(rw.Path())
	target := util.JoinURL(pr.U, path)
	accessLog.Target = target
	method := string(rw.Method())
	header := make(http.Header)

	rw.Request.Header.VisitAllInOrder(func(k, v []byte) {
		switch key := string(k); key {
		case "Content-Type", "Content-Encoding":
			header.Set(key, string(v))
		default:
			if ss.AnyOf(key, p.PassThroughHeader...) {
				header.Set(key, string(v))
			}
		}
	})

	for k, v := range pr.Header {
		header.Set(k, v)
	}

	var fn func(k, v []byte)
	if first {
		fn = func(k, v []byte) {
			if ss.AnyOf(string(k), "Content-Length", "Content-Encoding") {
				return
			}

			rw.RequestCtx.Response.Header.Add(string(k), string(v))
		}
	}

	code, rsp, err := util.TimeoutInvoke(target, method, header, string(body), pr.Timeout, fn)
	if err != nil {
		log.Printf("E! rest %s do failed: %v", target, err)
		return
	}

	accessLog.StatusCode = code
	accessLog.ResponseBody = rsp

	if !first {
		log.Printf("rest %s status: %d, rsp body: %s", target, code, rsp)
		return
	}

	if !util.InRange(accessLog.StatusCode, 200, 500) {
		return
	}

	if accessLog.StatusCode < 300 && !ss.AnyOf(method, "GET", "HEAD") && p.ch != nil {
		rb := model.Bean{
			Host:       string(rw.Host()),
			RemoteAddr: rw.RemoteAddr().String(),
			Method:     method,
			RequestURI: path,
			Labels:     p.Labels,
			Body:       string(body),
			Header:     http.Header{},
			ClusterIds: []string{pr.ClusterID},
		}
		rb.Header.Set("Content-Type", string(rw.Request.Header.Peek("Content-Type")))
		p.ch <- rb
	}

	rw.WriteHeader(code)
	if _, err := rw.Write([]byte(rsp)); err != nil {
		log.Printf("E! write data failed: %v", err)
	}
}
