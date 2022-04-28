package model

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/bingoohuang/gg/pkg/yaml"
)

type BackupBean struct {
	Body []byte
	Req  *http.Request
}

type Initializer interface {
	Initialize() error
}

type BackupWriter interface {
	Name() string
	Write(ctx context.Context, v BackupBean) error
}

type Elastic struct {
	Disabled bool
	URL      string
}

type Kafka struct {
	Disabled bool
	RawBody  bool
	Version  string
	Brokers  []string
	Topic    string
	Codec    string
	WarnSize int
}

type Config struct {
	Port    int
	Primary Elastic
	Backups []Elastic
	Kafkas  []Kafka
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
