package dest

import (
	"context"
	"fmt"
	"log"

	"github.com/Shopify/sarama"
	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/elasticproxy/pkg/util"
	"github.com/bingoohuang/gg/pkg/ginx"
	"github.com/bingoohuang/gg/pkg/ss"
)

type Kafka struct {
	model.KafkaDestination

	producer *Producer
	util.EvalLabels
}

func (d *Kafka) Name() string {
	return fmt.Sprintf("kafka")
}

func (d *Kafka) Initialize(context.Context) error {
	var err error
	d.EvalLabels, err = util.ParseLabelsExpr(d.LabelEval)
	if err != nil {
		return err
	}

	if len(d.Brokers) == 0 {
		return fmt.Errorf("brokers must be specified")
	}

	d.WarnSize = ss.Ori(d.WarnSize, 3*1024*1024) // 3 MiB

	// Apache KafkaDestination: Topic Naming Conventions
	// https://devshawn.com/blog/apache-kafka-topic-naming-conventions/
	// <data-center>.<domain>.<classification>.<description>.<version>

	kc := &ProducerConfig{Topic: ss.Or(d.Topic, "elastic.sync"), Version: d.Version, Brokers: d.Brokers, Codec: d.Codec}
	producer, err := kc.NewSyncProducer()
	if err != nil {
		return fmt.Errorf("failed to create kafka sync producer %w", err)
	}

	d.producer = producer
	return nil
}

func (d *Kafka) Write(ctx context.Context, bean model.Bean) error {
	vv, err := ginx.JsoniConfig.Marshal(ctx, bean)
	if err != nil {
		log.Printf("marshaling json %+v failed: %v", vv, err)
	}
	vLen := len(vv)

	prefix := ""
	if d.WarnSize > 0 && vLen >= d.WarnSize {
		prefix = "W!"
	} else {
		prefix = fmt.Sprintf("[L:15s]")
	}

	log.Printf("%s kafka write size: %d, message: %s,to kafka", prefix, vLen, vv)

	rsp, err := d.producer.Publish(d.Topic, vv)
	if err != nil {
		return fmt.Errorf("failed to publish len: %d, error %w, message: %s", vLen, err, vv)
	}

	log.Printf("%s kafka.produce result %+v", prefix, rsp)
	return nil
}

type Producer struct {
	SyncProducer sarama.SyncProducer
}

type ProducerConfig struct {
	Topic   string
	Version string
	Brokers []string
	Codec   string

	model.TlsConfig
}

type PublishResponse struct {
	Partition int32
	Offset    int64
	Topic     string
}

type Options struct {
	MessageKey string
	Headers    map[string]string
}

func (o *Options) Fulfil(msg *sarama.ProducerMessage) {
	if len(o.Headers) > 0 {
		msg.Headers = make([]sarama.RecordHeader, 0, len(o.Headers))
		for k, v := range o.Headers {
			msg.Headers = append(msg.Headers, sarama.RecordHeader{Key: []byte(k), Value: []byte(v)})
		}
	}

	if len(o.MessageKey) > 0 {
		msg.Key = sarama.StringEncoder(o.MessageKey)
	}
}

type OptionFn func(*Options)

func WithMessageKey(key string) OptionFn { return func(options *Options) { options.MessageKey = key } }
func WithHeader(k, v string) OptionFn    { return func(options *Options) { options.Headers[k] = v } }

func (p Producer) Publish(topic string, vv []byte, optionsFns ...OptionFn) (*PublishResponse, error) {
	options := &Options{Headers: make(map[string]string)}
	for _, f := range optionsFns {
		f(options)
	}

	// We are not setting a message key, which means that all messages will
	// be distributed randomly over the different partitions.
	msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(vv)}
	options.Fulfil(msg)

	partition, offset, err := p.SyncProducer.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	return &PublishResponse{Partition: partition, Offset: offset, Topic: msg.Topic}, nil
}

func (c ProducerConfig) NewSyncProducer() (*Producer, error) {
	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	sc := sarama.NewConfig()
	if err := util.ParseVersion(sc, c.Version); err != nil {
		return nil, err
	}

	sc.Producer.Compression = util.ParseCodec(c.Codec)
	sc.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	sc.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	sc.Producer.Return.Successes = true
	sc.Producer.MaxMessageBytes = int(sarama.MaxRequestSize)
	if tc := c.TlsConfig.Create(); tc != nil {
		sc.Net.TLS.Config = tc
		sc.Net.TLS.Enable = true
	}

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	p, err := sarama.NewSyncProducer(c.Brokers, sc)
	if err != nil {
		return nil, fmt.Errorf("failed to start Sarama SyncProducer, %w", err)
	}

	return &Producer{SyncProducer: p}, nil
}
