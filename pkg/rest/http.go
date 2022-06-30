package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bingoohuang/gg/pkg/codec"
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

	if b.AccessLog {
		defer func() {
			accessLog.Duration = time.Now().Sub(startTime).String()
			accessLog.StatusCode = status

			log.Printf("access log: %s", codec.Json(accessLog))
		}()
	}

	header := make(http.Header)
	for k, kv := range bean.Header {
		for _, vv := range kv {
			header.Add(k, vv)
		}
	}
	for k, v := range b.Header {
		header.Set(k, v)
	}

	status, rspBody, err := util.TimeoutInvoke(target, bean.Method, header, bean.Body, b.Timeout, nil)
	if err != nil {
		log.Printf("E! client do failed: %v", err)
		return err
	}

	accessLog.ResponseBody = rspBody

	return nil
}

func (b *Rest) InitializePrimary(_ context.Context) error {
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
	_, rsp, err := util.TimeoutGet(target, b.Timeout, b.Header)
	if err != nil {
		return err
	}

	if rsp == "" {
		return fmt.Errorf("no response body")
	}

	result := jj.Get(rsp, "_source.id")
	if result.Type == jj.String {
		b.ClusterID = result.String()
		return nil
	}

	clusterID := ksuid.New().String()
	data, _ := json.Marshal(map[string]string{"id": clusterID})
	status, rsp, err := util.TimeoutPost(target, "application/json", data, b.Timeout, b.Header)
	if err != nil {
		return err
	}

	log.Printf("create ClusterID, StatusCode: %d, postRspBody: %s ", status, rsp)

	if status != 201 {
		return fmt.Errorf("failed to create ClusterID at %s, StatusCode: %d, postRspBody: %s ", target, status, rsp)
	}
	b.ClusterID = clusterID
	return nil
}
