package kafkabackup

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/Shopify/sarama"
)

type Kafka struct {
	Version     string
	Brokers     []string
	TopicPrefix string
	Codec       string
	WarnSize    int

	producer *Producer
}

type Producer struct {
	SyncProducer sarama.SyncProducer
	Topic        string
}

type ProducerConfig struct {
	Topic   string
	Version string
	Brokers []string
	Codec   string

	TlsConfig
}

type PublishResponse struct {
	Partition int32  `json:"partition"`
	Offset    int64  `json:"offset"`
	Topic     string `json:"topic"`
}

type Options struct {
	MessageKey string
	Headers    map[string]string
}

func (o *Options) Fulfil(msg *sarama.ProducerMessage) {
	if len(o.Headers) > 0 {
		msg.Headers = make([]sarama.RecordHeader, 0, len(o.Headers))
		for k, v := range o.Headers {
			msg.Headers = append(msg.Headers, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(v),
			})
		}
	}

	if len(o.MessageKey) > 0 {
		msg.Key = sarama.StringEncoder(o.MessageKey)
	}
}

type OptionFn func(*Options)

func WithMessageKey(key string) OptionFn {
	return func(options *Options) {
		options.MessageKey = key
	}
}

func WithHeader(k, v string) OptionFn {
	return func(options *Options) {
		options.Headers[k] = v
	}
}

func (p Producer) Publish(topicPostfix string, vv []byte, optionsFns ...OptionFn) (*PublishResponse, error) {
	options := &Options{Headers: make(map[string]string)}
	for _, f := range optionsFns {
		f(options)
	}

	// We are not setting a message key, which means that all messages will
	// be distributed randomly over the different partitions.
	msg := &sarama.ProducerMessage{
		Topic: p.Topic + topicPostfix,
		Value: sarama.ByteEncoder(vv),
	}
	options.Fulfil(msg)

	partition, offset, err := p.SyncProducer.SendMessage(msg)
	if err != nil {
		return nil, err
	}

	return &PublishResponse{Partition: partition, Offset: offset, Topic: msg.Topic}, nil
}

func ParseCodec(codec string) sarama.CompressionCodec {
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

func (c ProducerConfig) NewSyncProducer() (*Producer, error) {
	// For the data collector, we are looking for strong consistency semantics.
	// Because we don't change the flush settings, sarama will try to produce messages
	// as fast as possible to keep latency low.
	config := sarama.NewConfig()
	if err := ParseVersion(config, c.Version); err != nil {
		return nil, err
	}

	config.Producer.Compression = ParseCodec(c.Codec)
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all in-sync replicas to ack the message
	config.Producer.Retry.Max = 10                   // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	config.Producer.MaxMessageBytes = int(sarama.MaxRequestSize)
	tlsConfig := c.CreateTlsConfig()
	if tlsConfig != nil {
		config.Net.TLS.Config = tlsConfig
		config.Net.TLS.Enable = true
	}

	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.
	p, err := sarama.NewSyncProducer(c.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to start Sarama SyncProducer, %w", err)
	}

	return &Producer{SyncProducer: p, Topic: c.Topic}, nil
}

type TlsConfig struct {
	CaFile, CertFile, KeyFile string
	InsecureSkipVerify        bool
}

func (tc TlsConfig) CreateTlsConfig() *tls.Config {
	if tc.CertFile != "" && tc.KeyFile != "" && tc.CaFile != "" {
		cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
		if err != nil {
			log.Fatal(err)
		}

		caCert, err := ioutil.ReadFile(tc.CaFile)
		if err != nil {
			log.Fatal(err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		return &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: tc.InsecureSkipVerify,
		}
	}

	// will be nil by default if nothing is provided
	return nil
}

func ParseVersion(config *sarama.Config, version string) error {
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
