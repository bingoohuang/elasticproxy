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
	GetLabels() map[string]any
}

type Destination struct {
	Primaries []rest.Rest
	Rests     []model.BackupWriter
	Kafkas    []model.BackupWriter
	stopCh    chan struct{}
}

func (d *Destination) Wait() {
	<-d.stopCh
}

func (d *Destination) Startup(ctx context.Context, ch chan model.Bean) {
	d.stopCh = make(chan struct{})
	for bean := range ch {
		writeBackups(ctx, d.Rests, bean)
		writeBackups(ctx, d.Kafkas, bean)
	}
	d.stopCh <- struct{}{}
}

func writeBackups(ctx context.Context, writers []model.BackupWriter, bean model.Bean) {
	for _, r := range writers {
		if ok, err := r.MatchLabels(bean.Labels); !ok {
			if err != nil {
				log.Printf("%s eval labels failed: %v", r.Name(), err)
			}
			continue
		}

		if err := model.RetryDo(ctx, func() error {
			return r.Write(ctx, bean)
		}); err != nil {
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
		if err := rr.InitializePrimary(ctx); err != nil {
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

func (s *Source) Stop() {
	for _, item := range s.Proxies {
		item.StopWait()
	}
	for _, item := range s.Kafkas {
		item.StopWait()
	}
}

func (s *Source) GoStartup(ctx context.Context, primaries []rest.Rest, ch chan<- model.Bean) error {
	for _, item := range s.Proxies {
		if i, ok := item.(model.Initializer); ok {
			if err := i.Initialize(ctx); err != nil {
				log.Printf("initialization failed: %v", err)
				return err
			}
		}
		go item.StartRead(ctx, primaries, ch)
	}

	for _, item := range s.Kafkas {
		if i, ok := item.(model.Initializer); ok {
			if err := i.Initialize(ctx); err != nil {
				log.Printf("initialization failed: %v", err)
				return err
			}
		}
		go item.StartRead(ctx, primaries, ch)
	}

	return nil
}

func CreateSources(config *model.Config) (*Source, error) {
	s := &Source{}
	for _, item := range config.Source.Proxies {
		if !item.Disabled {
			s.Proxies = append(s.Proxies, &source.ElasticProxy{ProxySource: item})
		}
	}
	for _, item := range config.Source.Kafkas {
		if !item.Disabled {
			s.Proxies = append(s.Proxies, &source.Kafka{KafkaSource: item})
		}
	}

	return s, nil
}
