package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/jj"
	"github.com/segmentio/ksuid"
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

func (b *Rest) Write(_ context.Context, bean model.Bean) error {
	if ss.AnyOf(b.ClusterID, bean.ClusterIds...) {
		log.Printf("already wrote to ClusterID %s, ignoring", b.ClusterID)
		return nil
	}

	status := 0
	target := util.JoinURL(b.U, bean.RequestURI)
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

	req, err := http.NewRequest(bean.Method, target, io.NopCloser(bytes.NewBuffer(bean.Body)))
	req.Header = bean.Header
	rsp, err := util.Client.Do(req)
	if err != nil {
		log.Printf("client do failed: %v", err)
		return err
	}
	status = rsp.StatusCode
	if rsp.Body != nil {
		_, _ = io.Copy(io.Discard, rsp.Body)
		iox.Close(rsp.Body)
	}
	return nil
}

func (b *Rest) MatchLabels(labels map[string]string) bool {
	ok, err := b.Eval(labels)
	if err != nil {
		log.Printf("eval labels failed: %v", err)
	}

	return ok
}

func (b *Rest) InitializePrimary(context.Context) error {
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

	log.Printf(" create ClusterID, StatusCode: %d, postRspBody: %s ", post.StatusCode, postRspBody)

	if post.StatusCode != 201 {
		return fmt.Errorf("failed to create ClusterID at %s, StatusCode: %d, postRspBody: %s ",
			target, post.StatusCode, postRspBody)
	}
	b.ClusterID = clusterID
	return nil
}
