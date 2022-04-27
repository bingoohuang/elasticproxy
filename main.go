package main

import (
	"fmt"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/golog"
	"log"
	"net/http"
	"net/url"
	"time"
)

var cliArgs struct {
	a    string
	b    []string
	port int
}

func main() {
	fla9.StringVar(&cliArgs.a, "primary,a", "http://127.0.0.1:9200/", "primary elastic URL")
	fla9.StringsVar(&cliArgs.b, "backups,b", nil, "backup elastic URLs")
	fla9.IntVar(&cliArgs.port, "port,p", 2900, "port to listen on")
	pInit := fla9.Bool("init", false, "Create initial ctl and exit")
	pVersion := fla9.Bool("version,v", false, "Create initial ctl and exit")
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
	golog.Setup()

	primaryElasticURL, err := url.Parse(cliArgs.a)
	if err != nil {
		log.Fatalf("Invalid URL %q: %s", cliArgs.a, err)
	}

	for _, b := range cliArgs.b {
		if ss.HasPrefix(b, "http:", "https:") {
			if _, err = url.Parse(b); err != nil {
				log.Fatalf("Invalid URL %q: %s", b, err)
			}
		}
	}

	// Set some more or less sensible limits & timeouts.
	http.DefaultTransport = &http.Transport{
		MaxIdleConns:          100,
		TLSHandshakeTimeout:   3 * time.Second,
		IdleConnTimeout:       15 * time.Minute,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	proxy := CreateElasticProxy(primaryElasticURL, cliArgs.b)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cliArgs.port), proxy); err != nil {
		log.Fatalf("ListenAndServe failed: %v", err)
	}
}
