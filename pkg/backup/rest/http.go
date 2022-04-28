package rest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/ginx"
)

type Rest struct {
	model.Elastic
	U *url.URL
}

func (b *Rest) Name() string {
	return fmt.Sprintf("elastic %s", b.U.String())
}

func (b *Rest) Initialize() error {
	u, err := url.Parse(b.URL)
	if err != nil {
		return err
	}
	b.U = u
	return nil
}

func (b *Rest) Write(ctx context.Context, bean model.BackupBean) error {
	status := 0
	target := util.JoinURL(b.U, bean.Req.RequestURI)
	fields := map[string]interface{}{
		"direction": "backup",
		"target":    target,
	}
	startTime := time.Now()

	defer func() {
		fields["duration"] = time.Now().Sub(startTime).String()
		fields["status"] = status

		accessLog, _ := ginx.JsoniConfig.MarshalToString(context.Background(), fields)
		log.Print(accessLog)
	}()

	req, err := http.NewRequest(bean.Req.Method, target, io.NopCloser(bytes.NewBuffer(bean.Body)))
	req.Header = bean.Req.Header
	rsp, err := util.Client.Do(req)
	if err != nil {
		log.Printf("client do failed: %v", err)
		return err
	}
	status = rsp.StatusCode
	if rsp.Body != nil {
		_, _ = io.Copy(io.Discard, rsp.Body)
	}
	return nil
}
