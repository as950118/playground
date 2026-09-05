// Optional second sink for the same metrics payload: instead of (or in
// addition to) Prometheus remote_write to Thanos, push the raw encoded
// WriteRequest bytes as a Kafka message. This lets us compare Kafka's
// producer-side compression codecs (none/gzip/snappy/lz4/zstd) against
// each other using real, small, high-frequency metrics payloads instead of
// a synthetic bulk-log payload.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
)

func newKafkaWriter(brokers []string, topic, compression string) (*kafka.Writer, error) {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 5 * time.Second,
		BatchSize:    200,
		RequiredAcks: kafka.RequireOne,
		// Async so a WriteMessages call enqueues and returns immediately
		// instead of blocking until the batch flushes. Without this, our
		// once-a-tick blocking call effectively serializes sends to one
		// per BatchTimeout, so each Kafka record batch ends up holding a
		// single message and compression never gets to work across a real
		// batch of samples.
		Async: true,
		Completion: func(messages []kafka.Message, err error) {
			if err != nil {
				log.Printf("kafka async write failed: %v", err)
			}
		},
	}

	switch strings.ToLower(compression) {
	case "", "none":
		// zero value: no compression
	case "gzip":
		w.Compression = compress.Gzip
	case "snappy":
		w.Compression = compress.Snappy
	case "lz4":
		w.Compression = compress.Lz4
	case "zstd":
		w.Compression = compress.Zstd
	default:
		return nil, fmt.Errorf("unknown KAFKA_COMPRESSION %q (want none/gzip/snappy/lz4/zstd)", compression)
	}

	return w, nil
}

func sendToKafka(ctx context.Context, w *kafka.Writer, payload []byte) error {
	return w.WriteMessages(ctx, kafka.Message{
		Value: payload,
		Time:  time.Now(),
	})
}
