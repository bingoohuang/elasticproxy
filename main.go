package main

import (
	"embed"
	"github.com/bingoohuang/gg/pkg/codec"
	"log"
	"net/http"
	"time"

	"github.com/bingoohuang/elasticproxy/pkg/process"
	"github.com/bingoohuang/gg/pkg/sigx"

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
		log.Fatalf("parse configuration, failed: %v", err)
	}

	log.Printf("read config: %s", codec.Json(c))

	ctx, _ := sigx.RegisterSignals(nil)
	sigx.RegisterSignalProfile()

	destinations, err := process.CreateDestinations(ctx, c)
	if err != nil {
		log.Fatalf("create destinations failed: %v", err)
	}

	sources, err := process.CreateSources(c)
	if err != nil {
		log.Fatalf("create sources failed: %v", err)
	}

	ch := make(chan model.Bean, c.ChanSize)
	if err := sources.GoStartup(ctx, destinations.Primaries, ch); err != nil {
		log.Fatalf("E! start up source failed: %v", err)
	}
	go destinations.Startup(ctx, ch)

	<-ctx.Done()

	sources.Stop()
	close(ch)
	destinations.Wait()
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
