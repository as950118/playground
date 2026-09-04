# go-agent

앞서 논의한 "자체 개발 Go agent로 실제 CPU/메모리/네트워크 사용량을 수집해서 Thanos Receive에 직접 넣는다"는 시나리오를 그대로 코드로 옮긴 것입니다. Prometheus는 이 경로에 전혀 등장하지 않습니다.

## 하는 일

1. [`gopsutil`](https://github.com/shirou/gopsutil)로 컨테이너(호스트)의 실제 CPU 사용률, 메모리 사용량, 네트워크 송수신 바이트를 10초마다 읽음
2. Prometheus remote_write 와이어 포맷(WriteRequest/TimeSeries/Label/Sample protobuf)을 직접 인코딩 (`remotewrite.go`) — `prometheus/prometheus` 전체를 의존성으로 끌어오지 않고 필요한 protobuf 타입만 손으로 구현
3. [`github.com/golang/snappy`](https://github.com/golang/snappy)(레퍼런스 구현)로 압축
4. `REMOTE_WRITE_URL`(기본 `http://thanos-receive-1:19291/api/v1/receive`)로 HTTP POST

라벨은 이름 기준 오름차순 정렬이 필요합니다(`remote_write` 스펙 요구사항 — 아니면 `409 out of order labels`). `encodeTimeSeries`에서 정렬을 강제합니다.

## 내보내는 메트릭

| 메트릭 | 의미 |
|---|---|
| `node_cpu_usage_percent` | 전체 CPU 사용률(%) |
| `node_memory_used_percent` | 메모리 사용률(%) |
| `node_memory_used_bytes` | 메모리 사용량(bytes) |
| `node_network_receive_bytes_total` | 누적 수신 바이트 (counter) |
| `node_network_transmit_bytes_total` | 누적 송신 바이트 (counter) |

모두 `job="go-custom-agent"`, `instance="go-agent-1"` 라벨이 붙습니다.

## 실행

`03-thanos-receive/docker-compose.yml`에 이미 포함되어 있습니다.

```bash
cd ../  # 03-thanos-receive/
docker compose up -d --build go-agent
docker compose logs -f go-agent
```

## 검증된 것

- `docker logs go-agent`에서 매 10초 `sent: cpu=... mem=... net_recv=... net_sent=...` 로그, HTTP 에러 없음
- Thanos Query로 실제 조회됨: `curl 'http://localhost:10904/api/v1/query?query={job="go-custom-agent"}'`
- counter 메트릭에 `rate()`도 정상 동작: `rate(node_network_receive_bytes_total{job="go-custom-agent"}[1m])`

즉 이 레포에서 손으로 만든 raw remote_write curl 데모(`docs/comparison.md` 참고)와 동일한 경로를, 실제 시스템 지표를 읽어서 주기적으로 보내는 진짜 프로그램으로 재현한 것입니다.
