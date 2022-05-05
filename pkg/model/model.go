package model

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bingoohuang/gg/pkg/backoff"

	"github.com/bingoohuang/elasticproxy/pkg/util"

	"github.com/bingoohuang/gg/pkg/yaml"
)

type Initializer interface {
	Initialize(ctx context.Context) error
}

type Bean struct {
	Labels     map[string]any
	Host       string
	RemoteAddr string
	Method     string
	RequestURI string
	Header     http.Header
	Body       string
	ClusterIds []string
}

type BackupBean struct {
	Body []byte
	Req  *http.Request
}

type BackupWriter interface {
	util.EvalLabels
	Name() string
	Write(ctx context.Context, v Bean) error
}

var ErrQuit = errors.New("quit")

func RetryWrite(ctx context.Context, f func() error) error {
	startTime := time.Now()

	b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx) // 创建 backoff 实例，使用指数级 bacckoff 算法
	operation := func(retryTimes int) error {                      // 需要重试的操作
		err := f()
		if err == ErrQuit { // 如果是无法重试的错误，使用 backoff.Permanent 包装，终止重试
			return backoff.Permanent(err)
		}
		return err
	}
	notify := func(retryTimes int, err error, d time.Duration) { // 发生重试的时候，通知回调函数
		if err == nil { // 重试成功
			log.Printf("W! recovered after retry times %d in %s", retryTimes, time.Since(startTime))
		} else { // 重试失败
			log.Printf("E! sleep %s for retry %d for error %v", d, retryTimes, err)
		}
	}

	return backoff.RetryNotify(operation, b, notify)
}

type Elastic struct {
	URL string

	Disabled  bool
	LabelEval string
}

type ProxySource struct {
	Port int

	Disabled bool
	Labels   map[string]any
}

func (p ProxySource) GetLabels() map[string]any { return p.Labels }

type KafkaSource struct {
	Version  string
	Brokers  []string
	Topics   []string
	Group    string
	Assignor string
	Newest   bool
	WarnSize int

	Disabled bool
	Labels   map[string]any
}

func (p KafkaSource) GetLabels() map[string]any { return p.Labels }

type KafkaDestination struct {
	Version  string
	Brokers  []string
	Topic    string
	Codec    string
	WarnSize int

	Disabled  bool
	LabelEval string
}

type Config struct {
	ChanSize int

	Source struct {
		Proxies []ProxySource
		Kafkas  []KafkaSource
	}

	Destination struct {
		Primaries []Elastic
		Rests     []Elastic
		Kafkas    []KafkaDestination
	}
}

func ParseConfFile(confFile string) (*Config, error) {
	confBytes, err := os.ReadFile(confFile)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %s error: %w", confFile, err)
	}

	ci := &Config{}
	if err := yaml.Unmarshal(confBytes, ci); err != nil {
		return nil, fmt.Errorf("decode yaml configInternal file %s error:%w", confFile, err)
	}

	return ci, nil
}

type TlsConfig struct {
	CaFile, CertFile, KeyFile string
	InsecureSkipVerify        bool
}

func (tc TlsConfig) Create() *tls.Config {
	if tc.CertFile == "" || tc.KeyFile == "" || tc.CaFile == "" {
		// will be nil by default if nothing is provided
		return nil
	}

	cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
	if err != nil {
		log.Fatal(err)
	}

	caCert, err := ioutil.ReadFile(tc.CaFile)
	if err != nil {
		log.Fatal(err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		InsecureSkipVerify: tc.InsecureSkipVerify,
	}
}

type AccessLog struct {
	RemoteAddr    string `json:",omitempty"`
	Method        string `json:",omitempty"`
	Path          string `json:",omitempty"`
	Target        string
	Direction     string
	XForwardedFor string `json:",omitempty"`
	Duration      string
	StatusCode    int
}
