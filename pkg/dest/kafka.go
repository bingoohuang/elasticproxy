package dest

import (
	"context"
	"fmt"
	"github.com/bingoohuang/gg/pkg/jsoni"
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
	util.LabelsMatcher

	Async        bool
	RequiredAcks string
}

func (d *Kafka) Hash() string { return util.Hash(d.Brokers...) }
func (d *Kafka) Name() string { return fmt.Sprintf("kafka") }

func (d *Kafka) Initialize(context.Context) error {
	var err error
	d.LabelsMatcher, err = util.ParseLabelsExpr(d.LabelEval)
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

	kc := &ProducerConfig{
		Topic:        ss.Or(d.Topic, "elastic.sync"),
		Version:      d.Version,
		Brokers:      d.Brokers,
		Codec:        d.Codec,
		Async:        d.Async,
		RequiredAcks: util.ParseRequiredAcks(d.RequiredAcks),
	}
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
		log.Printf("E! marshaling json %+v failed: %v", vv, err)
	}
	vLen := len(vv)

	prefix := ""
	if !d.NoAccessLog {
		if d.WarnSize > 0 && vLen >= d.WarnSize {
			prefix = "W!"
		} else {
			prefix = fmt.Sprintf("[L:15s]")
		}

		log.Printf("%s kafka write size: %d, message: %j,to kafka", prefix, vLen, jsoni.AsClearJSON(vv))
	}

	rsp, err := d.producer.Publish(d.Topic, vv)
	if err != nil {
		return fmt.Errorf("failed to publish len: %d, error %w, message: %s", vLen, err, vv)
	}

	if !d.NoAccessLog {
		log.Printf("%s kafka.produce result %j", prefix, jsoni.AsClearJSON(rsp))
	}
	return nil
}

type Producer struct {
	kafkaProducer
}

type ProducerConfig struct {
	Topic   string
	Version string
	Brokers []string
	Codec   string
	Async   bool
	sarama.RequiredAcks

	model.TlsConfig
}

type PublishResponse struct {
	Result interface{}
	Topic  string
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

type kafkaProducer interface {
	SendMessage(msg *sarama.ProducerMessage) (interface{}, error)
}

type asyncProducer struct {
	Producer sarama.AsyncProducer
}

func (p asyncProducer) SendMessage(msg *sarama.ProducerMessage) (interface{}, error) {
	p.Producer.Input() <- msg
	return AsyncProducerResult{Enqueued: true}, nil
}

type syncProducer struct {
	Producer sarama.SyncProducer
}

type AsyncProducerResult struct {
	Enqueued    bool
	ContextDone bool
}
type SyncProducerResult struct {
	Partition int32
	Offset    int64
}

func (p syncProducer) SendMessage(msg *sarama.ProducerMessage) (interface{}, error) {
	partition, offset, err := p.Producer.SendMessage(msg)
	return SyncProducerResult{Partition: partition, Offset: offset}, err
}

func (p Producer) Publish(topic string, vv []byte, optionsFns ...OptionFn) (*PublishResponse, error) {
	options := &Options{Headers: make(map[string]string)}
	for _, f := range optionsFns {
		f(options)
	}

	// We are not setting a message key, which means that all messages will
	// be distributed randomly over the different partitions.
	msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(vv)}
	options.Fulfil(msg)

	result, err := p.kafkaProducer.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	return &PublishResponse{Result: result, Topic: msg.Topic}, nil
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
	sc.Producer.RequiredAcks = c.RequiredAcks
	sc.Producer.Retry.Max = 3 // Retry up to x times to produce the message
	sc.Producer.MaxMessageBytes = int(sarama.MaxRequestSize)
	if tc := c.TlsConfig.Create(); tc != nil {
		sc.Net.TLS.Config = tc
		sc.Net.TLS.Enable = true
	}

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	if !c.Async {
		// Producer.Return.Successes must be true to be used in a SyncProducer
		sc.Producer.Return.Successes = true
		p, err := sarama.NewSyncProducer(c.Brokers, sc)
		if err != nil {
			return nil, fmt.Errorf("failed to start Sarama SyncProducer, %w", err)
		}
		return &Producer{kafkaProducer: &syncProducer{Producer: p}}, nil
	}

	sc.Producer.Return.Successes = false
	sc.Producer.Return.Errors = true
	p, err := sarama.NewAsyncProducer(c.Brokers, sc)
	if err != nil {
		return nil, fmt.Errorf("failed to start Sarama NewAsyncProducer, %w", err)
	}
	// We will just log to STDOUT if we're not able to produce messages.
	// Note: messages will only be returned here after all retry attempts are exhausted.
	go func() {
		for err := range p.Errors() {
			log.Println("Failed to write access log entry:", err)
		}
	}()

	return &Producer{kafkaProducer: &asyncProducer{Producer: p}}, nil
}
