package model

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/bingoohuang/elasticproxy/pkg/util"

	"github.com/bingoohuang/gg/pkg/yaml"
)

type Initializer interface {
	Initialize(ctx context.Context) error
}

type NameAware interface {
	Name() string
}

type Bean struct {
	Labels     map[string]any
	Host       string
	RemoteAddr string
	Method     string
	RequestURI string
	Header     http.Header
	Body       json.RawMessage
	ClusterIds []string
}

type BackupBean struct {
	Body []byte
	Req  *http.Request
}

type BackupWriter interface {
	util.EvalLabels
	NameAware
	Write(ctx context.Context, v Bean) error
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
