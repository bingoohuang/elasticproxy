package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bingoohuang/elasticproxy/pkg/backup/rest"
	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/golog"
)

func main() {
	pInit := fla9.Bool("init", false, "Create initial ctl and exit")
	pVersion := fla9.Bool("version,v", false, "Create initial ctl and exit")
	confFile := fla9.String("conf,c", "./conf.yml", "config file")
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion, InitFiles: &InitAssets}.ProcessInit()
	golog.Setup()

	c, err := model.ParseConfFile(*confFile)
	if err != nil {
		fmt.Printf("failed to parse configuration, error: %v", err)
		os.Exit(1)
	}

	primary := &rest.Rest{Elastic: c.Primary}
	if primary.Initialize() != nil {
		log.Fatalf("Initialize primary failed: %v", err)
	}

	proxy := CreateElasticProxy(context.Background(), primary, c)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", c.Port), proxy); err != nil {
		log.Fatalf("ListenAndServe failed: %v", err)
	}
}

func init() {
	// Set some more or less sensible limits & timeouts.
	http.DefaultTransport = &http.Transport{
		MaxIdleConns:          100,
		TLSHandshakeTimeout:   3 * time.Second,
		IdleConnTimeout:       15 * time.Minute,
		ResponseHeaderTimeout: 15 * time.Second,
	}
}

// InitAssets is the initial assets.
//go:embed initassets
var InitAssets embed.FS
