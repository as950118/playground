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

## Kafka sink (압축 코덱 비교용)

`../../practice-kafka` 클러스터의 압축 코덱(none/gzip/snappy/lz4/zstd)을 실제 메트릭 payload로 비교하기 위해, remote_write와 별개로 같은 인코딩된 payload를 Kafka로도 보낼 수 있습니다(`kafkasink.go`).

| 환경변수 | 기본값 | 설명 |
|---|---|---|
| `SEND_TARGET` | `remote_write` | `remote_write` \| `kafka` \| `both` |
| `BATCH_SIZE` | `1` | 이 틱 수만큼 모아서 한 시계열에 샘플을 여러 개(라벨은 한 번만) 실어 보냄. `remote_write`/`kafka` 양쪽 다 적용 |
| `KAFKA_BROKERS` | `kafka-1:29092,kafka-2:29092` | 콤마 구분 브로커 목록 |
| `KAFKA_TOPIC` | `go-agent-metrics` | 전송할 토픽 |
| `KAFKA_COMPRESSION` | `none` | `none` \| `gzip` \| `snappy` \| `lz4` \| `zstd` |

`BATCH_SIZE`는 remote_write에도 그대로 적용됩니다 — `WriteRequest`의 `TimeSeries`가 원래 샘플을 여러 개 가질 수 있는 구조라(`repeated Sample`), 라벨을 반복하지 않고 값·타임스탬프만 쌓아 보낼 수 있습니다. `BATCH_SIZE=1`(기본값)이면 기존과 동일하게 매 틱 개별 전송, `BATCH_SIZE=5`면 5틱마다 한 번, 5개 샘플이 든 요청을 보냅니다.

`docker-compose.yml`의 `go-agent` 서비스는 `practice-kafka`가 만든 외부 도커 네트워크(`practice-kafka_kafka-net`)에도 연결되어 있어, `practice-kafka`를 먼저 띄워두면 별도 설정 없이 붙습니다.

**주의**: `kafka.Writer`는 기본이 동기 호출이라, 매 tick마다 블로킹하며 보내면 배치(`BatchTimeout=5s`)가 flush될 때까지 다음 tick이 버려져서 배치당 메시지가 1개로 찌그러집니다(압축 효과가 사라짐). `Async: true` + `Completion` 콜백으로 우회했습니다 — 배치 압축 효과를 실측하려면 이 부분이 핵심입니다.

실측 결과와 압축 코덱 선택 가이드는 `practice-kafka/docs/03-compression.md`의 "실전 사례" 섹션 참고.
