package kaf

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/bingoohuang/gg/pkg/jsoni"
	"github.com/bingoohuang/jj"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/bingoohuang/elasticproxy/pkg/model"
	"github.com/bingoohuang/gg/pkg/ginx"
	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/Shopify/sarama"
)

type Kafka struct {
	model.Kafka

	producer *Producer
}

func (d *Kafka) Name() string {
	return fmt.Sprintf("backup to kafka")
}

func (d *Kafka) Initialize() error {
	if len(d.Brokers) == 0 {
		return fmt.Errorf("brokers must be specified")
	}

	d.WarnSize = ss.Ori(d.WarnSize, 3*1024*1024) // 3 MiB

	// Apache Kafka: Topic Naming Conventions
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

type ReqBean struct {
	Host       string
	RemoteAddr string
	Method     string
	URL        string
	Header     http.Header
	Body       interface{}
}

func (d *Kafka) Write(ctx context.Context, bean model.BackupBean) error {
	rb := ReqBean{
		Host:       bean.Req.Host,
		RemoteAddr: bean.Req.RemoteAddr,
		Method:     bean.Req.Method,
		URL:        bean.Req.RequestURI,
		Header:     bean.Req.Header,
	}

	if d.RawBody && jj.ParseBytes(bean.Body).IsObject() {
		rb.Body = jsoni.RawMessage(bean.Body)
	} else {
		rb.Body = string(bean.Body)
	}

	vv, err := ginx.JsoniConfig.Marshal(ctx, rb)
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

	TlsConfig
}

type TlsConfig struct {
	CaFile, CertFile, KeyFile string
	InsecureSkipVerify        bool
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
	if err := parseVersion(sc, c.Version); err != nil {
		return nil, err
	}

	sc.Producer.Compression = parseCodec(c.Codec)
	sc.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	sc.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	sc.Producer.Return.Successes = true
	sc.Producer.MaxMessageBytes = int(sarama.MaxRequestSize)
	if tc := c.CreateTlsConfig(); tc != nil {
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

func (tc TlsConfig) CreateTlsConfig() *tls.Config {
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

func parseVersion(config *sarama.Config, version string) error {
	if version == "" {
		return nil
	}

	v, err := sarama.ParseKafkaVersion(version)
	if err != nil {
		return fmt.Errorf("parsing Kafka version error: %w", err)
	}

	config.Version = v
	return nil
}

func parseCodec(codec string) sarama.CompressionCodec {
	switch l := strings.ToLower(strings.TrimSpace(codec)); l {
	case "none":
		return sarama.CompressionNone
	case "gzip":
		return sarama.CompressionGZIP
	case "snappy":
		return sarama.CompressionSnappy
	case "lz4":
		return sarama.CompressionLZ4
	case "zstd":
		return sarama.CompressionZSTD
	default:
		log.Printf("W! unknown compression codec %s", codec)
		return sarama.CompressionNone
	}
}
