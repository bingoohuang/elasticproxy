package process

import (
	"context"
	"log"

	"github.com/bingoohuang/elasticproxy/pkg/dest"
	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/rest"
	"github.com/bingoohuang/elasticproxy/pkg/source"
)

type SourceReader interface {
	StartRead(ctx context.Context, primaries []rest.Rest, ch chan<- model.Bean)
	StopWait()
	GetLabels() map[string]string
}

type Destination struct {
	Primaries []rest.Rest
	Rests     []model.BackupWriter
	Kafkas    []model.BackupWriter
}

func (d Destination) Startup(ctx context.Context, ch chan model.Bean) {
	for bean := range ch {
		writeBackups(ctx, d.Rests, bean)
		writeBackups(ctx, d.Kafkas, bean)
	}
}

func writeBackups(ctx context.Context, writers []model.BackupWriter, bean model.Bean) {
	for _, r := range writers {
		if ok, err := r.Eval(bean.Labels); !ok {
			if err != nil {
				log.Printf("%s eval labels failed: %v", r.Name(), err)
			}
			continue
		}
		if err := r.Write(ctx, bean); err != nil {
			log.Printf("write %s failed: %v", r.Name(), err)
		}
	}
}

func CreateDestinations(ctx context.Context, config *model.Config) (*Destination, error) {
	d := &Destination{}
	for _, item := range config.Destination.Primaries {
		if item.Disabled {
			continue
		}

		rr := rest.Rest{Elastic: item}
		if err := rr.Initialize(ctx); err != nil {
			return nil, err
		}

		d.Primaries = append(d.Primaries, rr)
	}

	for _, item := range config.Destination.Rests {
		if item.Disabled {
			continue
		}

		rr := &rest.Rest{Elastic: item}
		if err := rr.Initialize(ctx); err != nil {
			return nil, err
		}
		d.Rests = append(d.Rests, rr)
	}

	for _, item := range config.Destination.Kafkas {
		if item.Disabled {
			continue
		}

		rr := &dest.Kafka{KafkaDestination: item}
		if err := rr.Initialize(ctx); err != nil {
			return nil, err
		}
		d.Kafkas = append(d.Kafkas, rr)
	}

	return d, nil
}

type Source struct {
	Proxies []SourceReader
	Kafkas  []SourceReader
}

func (s *Source) GoStartup(ctx context.Context, primaries []rest.Rest, ch chan<- model.Bean) {
	for _, item := range s.Proxies {
		if i, ok := item.(model.Initializer); ok {
			if err := i.Initialize(ctx); err != nil {
				log.Printf("initialization failed: %v", err)
			}
		}
		go item.StartRead(ctx, primaries, ch)
	}

	for _, item := range s.Kafkas {
		if i, ok := item.(model.Initializer); ok {
			if err := i.Initialize(ctx); err != nil {
				log.Printf("initialization failed: %v", err)
			}
		}
		go item.StartRead(ctx, primaries, ch)
	}
}

func CreateSources(config *model.Config) (*Source, error) {
	s := &Source{}
	for _, item := range config.Source.Proxies {
		if item.Disabled {
			continue
		}

		p := &source.ElasticProxy{ProxySource: item}
		s.Proxies = append(s.Proxies, p)
	}
	for _, item := range config.Source.Kafkas {
		if item.Disabled {
			continue
		}

		p := &source.Kafka{KafkaSource: item}
		s.Proxies = append(s.Proxies, p)
	}

	return s, nil
}
