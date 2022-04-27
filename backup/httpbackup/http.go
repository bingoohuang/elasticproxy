package httpbackup

import (
	"bytes"
	"context"
	"github.com/bingoohuang/elasticproxy/backup/model"
	"github.com/bingoohuang/elasticproxy/backup/util"
	"github.com/bingoohuang/gg/pkg/ginx"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type backup struct {
	URL *url.URL
}

func NewBackup(backupURL string) model.Backup {
	u, _ := url.Parse(backupURL)
	return &backup{URL: u}
}

func (b backup) BackupOne(bean model.BackupBean) {
	status := 0
	target := util.JoinURL(b.URL, bean.Req.RequestURI)
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
		return
	}
	status = rsp.StatusCode
	if rsp.Body != nil {
		_, _ = io.Copy(io.Discard, rsp.Body)
	}
}
