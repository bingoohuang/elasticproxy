package util

import (
	"fmt"
	"log"
	"strings"

	"github.com/Shopify/sarama"
)

func ParseVersion(config *sarama.Config, version string) error {
	if version == "" {
		return nil
	}

	v, err := sarama.ParseKafkaVersion(version)
	if err != nil {
		return fmt.Errorf("parsing KafkaDestination version error: %w", err)
	}

	config.Version = v
	return nil
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

func ParseBalanceStrategy(assignor string) sarama.BalanceStrategy {
	switch assignor {
	case "sticky":
		return sarama.BalanceStrategySticky
	case "roundrobin", "rr":
		return sarama.BalanceStrategyRoundRobin
	case "range":
		return sarama.BalanceStrategyRange
	default:
		return sarama.BalanceStrategyRange
	}
}

func ParseRequiredAcks(acks string) sarama.RequiredAcks {
	acks = strings.ToLower(acks)
	switch acks {
	case "waitforlocal":
		return sarama.WaitForLocal
	case "noresponse":
		return sarama.NoResponse
	case "waitforall":
		return sarama.WaitForAll
	default:
		return sarama.WaitForLocal
	}
}
