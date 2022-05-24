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

	util.LabelsMatcher
	ClusterID string
}

func (b *Rest) Hash() string { return util.HashURL(b.U) }
func (b *Rest) Name() string { return fmt.Sprintf("elastic %s", b.U.String()) }

func (b *Rest) Initialize(context.Context) error {
	var err error
	b.LabelsMatcher, err = util.ParseLabelsExpr(b.LabelEval)
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
		log.Printf("rest, already wrote to ClusterID %s, ignoring", b.ClusterID)
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

	if !b.NoAccessLog {
		defer func() {
			accessLog.Duration = time.Now().Sub(startTime).String()
			accessLog.StatusCode = status

			log.Printf("access log: %s", codec.Json(accessLog))
		}()
	}

	req, _ := http.NewRequestWithContext(ctx, bean.Method, target, io.NopCloser(strings.NewReader(bean.Body)))
	req.Header = bean.Header
	for k, v := range b.Header {
		req.Header.Add(k, v)
	}

	rsp, err := util.TimeoutInvoke(req, b.Timeout)
	if err != nil {
		log.Printf("E! client do failed: %v", err)
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

func (b *Rest) InitializePrimary(ctx context.Context) error {
	u := *b.U

	// https://discuss.elastic.co/t/index-name-type-name-and-field-name-rules/133039
	// The rules for index names are encoded in MetaDataCreateIndexService 730. Essentially:
	//
	// 1. Lowercase only
	// 2. Cannot include \, /, *, ?, ", <, >, |, space (the character, not the word), ,, #
	// 3. Indices prior to 7.0 could contain a colon (:), but that's been deprecated and won't be supported in 7.0+
	// 4. Cannot start with -, _, +
	// 5. Cannot be . or ..
	// 6. Cannot be longer than 255 characters
	u.Path = "/meta.elasticproxy/_doc/clusterid"
	target := u.String()
	rsp, err := util.TimeoutGet(ctx, target, b.Timeout, b.Header)
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
	post, err := util.TimeoutPost(ctx, target, "application/json",
		io.NopCloser(bytes.NewReader(data)), b.Timeout, b.Header)
	if err != nil {
		return err
	}

	postRspBody, _ := util.ReadBody(post)
	log.Printf("create ClusterID, StatusCode: %d, postRspBody: %s ", post.StatusCode, postRspBody)

	if post.StatusCode != 201 {
		return fmt.Errorf("failed to create ClusterID at %s, StatusCode: %d, postRspBody: %s ",
			target, post.StatusCode, postRspBody)
	}
	b.ClusterID = clusterID
	return nil
}
