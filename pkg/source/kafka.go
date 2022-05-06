package source

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/jj"

	"github.com/Shopify/sarama"
	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/rest"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/ginx"
	"github.com/bingoohuang/gg/pkg/ss"
)

type Kafka struct {
	model.KafkaSource

	client sarama.ConsumerGroup
	cs     *consumer
}

func (s *Kafka) Initialize(context.Context) error {
	c := sarama.NewConfig()
	if err := util.ParseVersion(c, s.Version); err != nil {
		return err
	}

	c.Consumer.Group.Rebalance.Strategy = util.ParseBalanceStrategy(s.Assignor)
	if !s.Newest {
		c.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	client, err := sarama.NewConsumerGroup(s.Brokers, s.Group, c)
	if err != nil {
		return fmt.Errorf("creating consumer group client: %w", err)
	}
	s.client = client

	return nil
}

func (s *Kafka) StopWait() {
	s.cs.StopWait()
}

func (s *Kafka) StartRead(ctx context.Context, primaries []rest.Rest, ch chan<- model.Bean) {
	s.cs = &consumer{
		ctx:      ctx,
		client:   s.client,
		out:      ch,
		group:    s.Group,
		topics:   s.Topics,
		warnSize: ss.Ori(s.WarnSize, 3*1024*1024), // 3 MiB
		labels:   s.Labels,
	}

	for _, primary := range primaries {
		if primary.MatchLabels(s.Labels) {
			s.cs.Primaries = append(s.cs.Primaries, primary)
		}
	}

	s.cs.consume()
}

func (c *consumer) StopWait() {
	log.Printf("kafka consumer %s for topic %s up and running...", c.group, c.topics)
	<-c.ctx.Done()
	if err := c.client.Close(); err != nil {
		log.Printf("E! closing client: %v", err)
	}
}

func (c *consumer) consume() {
	// check if context was cancelled, signaling that the consumer should stop
	for c.ctx.Err() == nil {
		// `Consume` should be called inside an infinite loop, when a
		// server-side re-balance happens, the consumer session will need to be
		// recreated to get the new claims
		if err := c.client.Consume(c.ctx, c.topics, c); err != nil {
			log.Printf("E! Error from consumer: %v", err)
		}
	}

	close(c.out)
}

// consumer represents a Sarama consumer group consumer
type consumer struct {
	out       chan<- model.Bean
	group     string
	warnSize  int
	ctx       context.Context
	client    sarama.ConsumerGroup
	topics    []string
	Primaries []rest.Rest
	labels    map[string]any
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (c *consumer) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (c *consumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (c *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) (err error) {
	// NOTE: Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for m := range claim.Messages() {
		valLen := len(m.Value)
		prefix := ss.If(c.warnSize > 0 && valLen >= c.warnSize, "W!", "[L:15s]")
		log.Printf("%s kafka.claimed group: %s, len: %d, value: %s, time: %v, topic: %s, offset: %d, partition: %d",
			prefix, c.group, valLen, m.Value, m.Timestamp, m.Topic, m.Offset, m.Partition)

		var bean model.Bean
		if err := ginx.JsoniConfig.Unmarshal(c.ctx, m.Value, &bean); err != nil {
			log.Printf("unmarshal %s failed: %v", m.Value, err)
		} else {
			bean.Labels = c.labels
			c.writePrimaries(bean)
			c.out <- bean
		}
		session.MarkMessage(m, "")
	}

	return nil
}

func (c *consumer) writePrimaries(bean model.Bean) {
	for _, primary := range c.Primaries {
		if primary.MatchLabels(c.labels) {
			if err := model.RetryDo(c.ctx, func() error {
				c.writePrimary(primary, bean)
				return nil
			}); err != nil {
				log.Printf("retry failed: %v", err)
			}
		}
	}
}

func (c *consumer) writePrimary(primary rest.Rest, bean model.Bean) {
	if ss.AnyOf(primary.ClusterID, bean.ClusterIds...) {
		log.Printf("already wrote to ClusterID %s, ignoring", primary.ClusterID)
		return
	}

	target := util.JoinURL(primary.U, bean.RequestURI)
	req, err := http.NewRequest(bean.Method, target, ioutil.NopCloser(strings.NewReader(bean.Body)))
	for k, vv := range bean.Header {
		for _, vi := range vv {
			req.Header.Add(k, vi)
		}
	}

	if primary.Timeout > 0 {
		ctx, cancel := context.WithTimeout(c.ctx, primary.Timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	rsp, err := util.Client.Do(req)
	if err != nil {
		log.Printf("rest %s do failed: %v", target, err)
		return
	}

	var data []byte
	if rsp.Body != nil {
		defer iox.Close(rsp.Body)
		if data, err = io.ReadAll(rsp.Body); err != nil {
			log.Printf("reading response body failed: %v", err)
		}
	}
	log.Printf("rest %s do status: %d, response: %s", target, rsp.StatusCode, jj.Ugly(data))
}
