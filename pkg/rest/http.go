package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/codec"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/jj"
	"github.com/segmentio/ksuid"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/util"
)

type Rest struct {
	model.Elastic
	U *url.URL

	util.EvalLabels
	ClusterID string
}

func (b *Rest) Name() string {
	return fmt.Sprintf("elastic %s", b.U.String())
}

func (b *Rest) Initialize(context.Context) error {
	var err error
	b.EvalLabels, err = util.ParseLabelsExpr(b.LabelEval)
	if err != nil {
		return err
	}

	u, err := url.Parse(b.URL)
	if err != nil {
		return err
	}
	b.U = u

	return nil
}

func (b *Rest) Write(ctx context.Context, bean model.Bean) error {
	if ss.AnyOf(b.ClusterID, bean.ClusterIds...) {
		log.Printf("already wrote to ClusterID %s, ignoring", b.ClusterID)
		return nil
	}

	status := 0
	target := util.JoinURL(b.U, bean.RequestURI)
	accessLog := model.AccessLog{
		RemoteAddr: bean.RemoteAddr,
		Method:     bean.Method,
		Path:       bean.RequestURI,
		Direction:  "backup",
		Target:     target,
	}
	startTime := time.Now()

	defer func() {
		accessLog.Duration = time.Now().Sub(startTime).String()
		accessLog.StatusCode = status

		log.Printf("access log: %s", codec.Json(accessLog))
	}()

	req, err := http.NewRequest(bean.Method, target, io.NopCloser(strings.NewReader(bean.Body)))
	req.Header = bean.Header

	if b.Timeout > 0 {
		ctx, cancel := context.WithTimeout(ctx, b.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	rsp, err := util.Client.Do(req)
	if err != nil {
		log.Printf("client do failed: %v", err)
		return err
	}
	status = rsp.StatusCode

	rspBody, err := util.ReadBody(rsp)
	if err != nil {
		return err
	}
	accessLog.ResponseBody = string(rspBody)

	return nil
}

func (b *Rest) MatchLabels(labels map[string]any) bool {
	ok, err := b.Eval(labels)
	if err != nil {
		log.Printf("eval labels failed: %v", err)
	}

	return ok
}

func (b *Rest) InitializePrimary(_ context.Context) error {
	u := *b.U
	u.Path = "/elasticproxy/doc/clusterid"
	target := u.String()

	rsp, err := util.Client.Get(target)
	if err != nil {
		return err
	}

	if rsp.Body == nil {
		return fmt.Errorf("no response body")
	}

	defer iox.Close(rsp.Body)

	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	result := jj.GetBytes(data, "_source.id")
	if result.Type == jj.String {
		b.ClusterID = result.String()
		return nil
	}

	clusterID := ksuid.New().String()
	data, _ = json.Marshal(map[string]string{"id": clusterID})
	post, err := util.Client.Post(target, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	var postRspBody []byte
	if post.Body != nil {
		defer iox.Close(post.Body)
		postRspBody, err = io.ReadAll(post.Body)
	}

	log.Printf("create ClusterID, StatusCode: %d, postRspBody: %s ", post.StatusCode, postRspBody)

	if post.StatusCode != 201 {
		return fmt.Errorf("failed to create ClusterID at %s, StatusCode: %d, postRspBody: %s ",
			target, post.StatusCode, postRspBody)
	}
	b.ClusterID = clusterID
	return nil
}
