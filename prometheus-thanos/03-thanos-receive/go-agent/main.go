// go-agent is a stand-in for a self-developed metrics collector: it reads
// real CPU/memory/network usage from the host it runs on and pushes it
// straight to Thanos Receive via the Prometheus remote_write wire protocol
// (protobuf + snappy), with no Prometheus involved anywhere in the path.
package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/segmentio/kafka-go"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

func main() {
	remoteWriteURL := getEnv("REMOTE_WRITE_URL", "http://thanos-receive-1:19291/api/v1/receive")
	interval := getEnvDuration("COLLECT_INTERVAL", 10*time.Second)
	instance := getEnv("AGENT_INSTANCE", hostname())
	job := getEnv("AGENT_JOB", "go-custom-agent")

	// SEND_TARGET picks the sink(s) for the same encoded metrics payload:
	// remote_write (default, unchanged behavior), kafka, or both at once.
	target := getEnv("SEND_TARGET", "remote_write")

	var kafkaWriter *kafka.Writer
	if target == "kafka" || target == "both" {
		brokers := strings.Split(getEnv("KAFKA_BROKERS", "kafka-1:29092,kafka-2:29092"), ",")
		topic := getEnv("KAFKA_TOPIC", "go-agent-metrics")
		compression := getEnv("KAFKA_COMPRESSION", "none")
		w, err := newKafkaWriter(brokers, topic, compression)
		if err != nil {
			log.Fatalf("kafka writer setup failed: %v", err)
		}
		defer w.Close()
		kafkaWriter = w
		log.Printf("kafka sink enabled: brokers=%v topic=%s compression=%s", brokers, topic, compression)
	}

	// BATCH_SIZE accumulates this many collection ticks into a single
	// WriteRequest (one TimeSeries per metric, multiple Samples each)
	// before sending, instead of firing one request per tick. remote_write
	// already supports multiple samples per series without repeating
	// labels, so this is a fairer comparison against Kafka's own batching
	// than the previous one-request-per-tick behavior. Defaults to 1
	// (unbatched) to keep existing remote_write-only usage unchanged.
	batchSize := getEnvInt("BATCH_SIZE", 1)

	log.Printf("go-agent starting: instance=%s job=%s interval=%s batch_size=%d target=%s -> %s",
		instance, job, interval, batchSize, target, remoteWriteURL)

	baseLabels := []label{
		{"job", job},
		{"instance", instance},
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	batch := newBatch(baseLabels)
	for {
		if err := collectInto(batch); err != nil {
			log.Printf("collect failed: %v", err)
		} else if batch.count() >= batchSize {
			if err := sendBatch(remoteWriteURL, target, kafkaWriter, batch); err != nil {
				log.Printf("send failed: %v", err)
			}
			batch = newBatch(baseLabels)
		}
		<-ticker.C
	}
}

// batch accumulates one growing TimeSeries per metric across multiple
// collection ticks; labels are set once and samples are appended.
type batch struct {
	metrics []series // index matches metricNames
}

var metricNames = []string{
	"node_cpu_usage_percent",
	"node_memory_used_percent",
	"node_memory_used_bytes",
	"node_network_receive_bytes_total",
	"node_network_transmit_bytes_total",
}

func newBatch(base []label) *batch {
	metrics := make([]series, len(metricNames))
	for i, name := range metricNames {
		metrics[i] = series{labels: append([]label{{"__name__", name}}, base...)}
	}
	return &batch{metrics: metrics}
}

func (b *batch) count() int {
	return len(b.metrics[0].samples)
}

func (b *batch) append(now int64, values ...float64) {
	for i, v := range values {
		b.metrics[i].samples = append(b.metrics[i].samples, sample{value: v, timestamp: now})
	}
}

func collectInto(b *batch) error {
	now := time.Now().UnixMilli()

	cpuPercents, err := cpu.Percent(0, false)
	if err != nil {
		return err
	}
	cpuUsage := 0.0
	if len(cpuPercents) > 0 {
		cpuUsage = cpuPercents[0]
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return err
	}

	netCounters, err := net.IOCounters(false)
	if err != nil {
		return err
	}
	var bytesRecv, bytesSent uint64
	if len(netCounters) > 0 {
		bytesRecv = netCounters[0].BytesRecv
		bytesSent = netCounters[0].BytesSent
	}

	b.append(now, cpuUsage, vmem.UsedPercent, float64(vmem.Used), float64(bytesRecv), float64(bytesSent))
	return nil
}

func sendBatch(url, target string, kafkaWriter *kafka.Writer, b *batch) error {
	// The same encoded WriteRequest bytes feed both sinks: remote_write
	// compresses it with snappy itself (Prometheus wire protocol default,
	// not a free choice), while the Kafka sink hands the uncompressed
	// bytes to kafka-go, which applies whichever KAFKA_COMPRESSION codec
	// the writer was built with.
	payload := encodeWriteRequest(b.metrics)
	// Computed unconditionally (cheap, small payloads) so every send logs
	// what remote_write's mandatory snappy would have produced for this
	// exact batch, whether or not this run actually targets remote_write —
	// gives a same-payload baseline to compare against Kafka's codec.
	snappyBytes := snappy.Encode(nil, payload)

	if target == "remote_write" || target == "both" {
		compressed := snappyBytes

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(compressed))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Encoding", "snappy")
		req.Header.Set("Content-Type", "application/x-protobuf")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode/100 != 2 {
			return &httpStatusError{resp.StatusCode}
		}
	}

	if target == "kafka" || target == "both" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := sendToKafka(ctx, kafkaWriter, payload); err != nil {
			return err
		}
	}

	last := b.count() - 1
	log.Printf("sent: batch=%d cpu=%.1f%% mem=%.1f%% mem_used=%.0fMB net_recv=%.0fB net_sent=%.0fB raw_bytes=%d snappy_bytes=%d",
		b.count(),
		b.metrics[0].samples[last].value,
		b.metrics[1].samples[last].value,
		b.metrics[2].samples[last].value/1024/1024,
		b.metrics[3].samples[last].value,
		b.metrics[4].samples[last].value,
		len(payload), len(snappyBytes))
	return nil
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string {
	return "remote write rejected with http " + strconv.Itoa(e.code)
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
