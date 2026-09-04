// go-agent is a stand-in for a self-developed metrics collector: it reads
// real CPU/memory/network usage from the host it runs on and pushes it
// straight to Thanos Receive via the Prometheus remote_write wire protocol
// (protobuf + snappy), with no Prometheus involved anywhere in the path.
package main

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang/snappy"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

func main() {
	remoteWriteURL := getEnv("REMOTE_WRITE_URL", "http://thanos-receive-1:19291/api/v1/receive")
	interval := getEnvDuration("COLLECT_INTERVAL", 10*time.Second)
	instance := getEnv("AGENT_INSTANCE", hostname())
	job := getEnv("AGENT_JOB", "go-custom-agent")

	log.Printf("go-agent starting: instance=%s job=%s interval=%s -> %s", instance, job, interval, remoteWriteURL)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := collectAndSend(remoteWriteURL, job, instance); err != nil {
			log.Printf("send failed: %v", err)
		}
		<-ticker.C
	}
}

func collectAndSend(url, job, instance string) error {
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

	baseLabels := []label{
		{"job", job},
		{"instance", instance},
	}

	metrics := []series{
		metric("node_cpu_usage_percent", baseLabels, cpuUsage, now),
		metric("node_memory_used_percent", baseLabels, vmem.UsedPercent, now),
		metric("node_memory_used_bytes", baseLabels, float64(vmem.Used), now),
		metric("node_network_receive_bytes_total", baseLabels, float64(bytesRecv), now),
		metric("node_network_transmit_bytes_total", baseLabels, float64(bytesSent), now),
	}

	payload := encodeWriteRequest(metrics)
	compressed := snappy.Encode(nil, payload)

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

	log.Printf("sent: cpu=%.1f%% mem=%.1f%% mem_used=%dMB net_recv=%dB net_sent=%dB",
		cpuUsage, vmem.UsedPercent, vmem.Used/1024/1024, bytesRecv, bytesSent)
	return nil
}

func metric(name string, base []label, value float64, timestampMs int64) series {
	labels := append([]label{{"__name__", name}}, base...)
	return series{
		labels:  labels,
		samples: []sample{{value: value, timestamp: timestampMs}},
	}
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
