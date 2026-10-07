# 압축 방식 비교 (none / gzip / snappy / lz4 / zstd)

## 동작 방식

프로듀서가 배치(batch) 단위로 압축해서 보내고, 브로커는 기본 설정(`compression.type=producer`)에서는 재압축 없이 그대로 저장했다가 컨슈머에게 그대로 넘깁니다. 토픽/브로커에 다른 `compression.type`을 강제하면 브로커가 다시 압축합니다.

압축 효율은 **배치 크기**에 크게 좌우됩니다. `linger.ms`/`batch.size`를 키워서 배치를 크게 만들수록 압축률이 좋아지고, 작은 메시지를 하나씩 압축하면 오히려 오버헤드가 더 클 수 있습니다.

## 정성적 특성

| 방식 | 압축률 | CPU 비용 | 비고 |
|---|---|---|---|
| `none` | - | 없음 | 최대 대역폭/디스크 사용. 극한 저지연 아니면 잘 안 씀 |
| `gzip` | 높음 | 가장 높음(느림) | 대역폭이 정말 부족하거나 콜드 아카이빙용 |
| `snappy` | 중간 | 낮음 | 예전 기본 추천값 |
| `lz4` | snappy와 비슷 | 가장 낮음 | 압축·해제 모두 제일 빠름. 처리량 우선 워크로드 |
| `zstd` (2.1+) | 동급 CPU 대비 최고 | 레벨 조절 가능 | 최신 권장값. gzip급 압축률 + gzip보다 빠름 |

## 실측 벤치마크

이 저장소의 3노드 클러스터에서 `kafka-producer-perf-test.sh`로 실제 측정했습니다.

**테스트 데이터**: JSON 로그 형태 이벤트 2만 건 (`event`, `user`, `timestamp`, `session_id` 등 필드, 반복 패턴이 있는 전형적인 로그성 텍스트 — 총 2.4MB)

**압축률 (디스크 실사용량 기준, `kafka-log-dirs.sh --describe`로 확인)**

| 압축 | 디스크 사용량 | compression-rate-avg | 원본 대비 |
|---|---|---|---|
| none | 2,594,798 B | 1.000 | 기준 |
| **zstd** | **578,070 B** | **0.222** | **약 4.5배 축소** |
| gzip | 587,704 B | 0.226 | 약 4.4배 축소 |
| lz4 | 893,015 B | 0.344 | 약 2.9배 축소 |
| snappy | 896,674 B | 0.346 | 약 2.9배 축소 |

`compression-rate-avg`는 프로듀서 자체 메트릭(`--print-metrics`)이고, 디스크 사용량은 브로커에 실제 저장된 바이트 수라 서로 교차 검증됨 — 결과가 일치했습니다.

**결론**: JSON처럼 반복 패턴이 많은 텍스트성 데이터에서는 **zstd/gzip이 snappy/lz4보다 압축률이 거의 2배** 좋습니다.

### 처리량(속도)은 왜 표에서 뺐나

같은 벤치마크에서 records/sec도 측정했지만, 이 실습 환경(Colima VM, CPU 2코어 + 다른 프로젝트 컨테이너 20개 이상 동시 실행)에서는 실행할 때마다 순위가 바뀔 정도로 노이즈가 컸습니다(zstd가 snappy보다 빠르게 나오는 등 이론과 안 맞는 흔들림). 압축률(바이트 수)은 CPU 스케줄링과 무관하게 결정적으로 나오므로 신뢰하되, 처리량 숫자는 이 환경에서 신뢰도가 낮다고 판단해 정성적 설명(위 표)으로 대체했습니다.

**속도 관련 검증된 사실**:
- `lz4` ≈ `snappy`: 압축 안 하는 것에 가까울 정도로 빠름, CPU 오버헤드 거의 없음
- `zstd`: lz4보다 느리지만 gzip급 압축률. `level` 파라미터로 속도⇄압축률 조절 가능(레벨 낮추면 lz4에 근접)
- `gzip`: 셋 중 가장 느림, 특히 압축 쪽

## 실무 선택 가이드

- **네트워크/디스크 비용 최우선** → `zstd` (gzip급 압축률 + gzip보다 빠름)
- **처리량/지연 최우선** → `lz4`
- `snappy`는 `lz4`보다 나을 게 없어서 최신 클러스터에서는 `lz4`/`zstd`로 대체되는 추세

## 재현 방법

```bash
# 코덱별 토픽 생성 후
docker exec kafka-1 /opt/kafka/bin/kafka-producer-perf-test.sh \
  --topic bench-<codec> \
  --num-records 20000 \
  --throughput -1 \
  --payload-file /tmp/kafka_bench_payload.txt \
  --producer-props bootstrap.servers=kafka-1:29092 acks=1 compression.type=<codec> batch.size=131072 linger.ms=20 \
  --print-metrics

# 디스크 사용량 비교
docker exec kafka-1 /opt/kafka/bin/kafka-log-dirs.sh --describe \
  --bootstrap-server kafka-1:29092 \
  --topic-list bench-none,bench-gzip,bench-snappy,bench-lz4,bench-zstd
```

## 실전 사례: 합성 데이터가 아니라 진짜 메트릭 payload로 재현

위 벤치마크는 `kafka-producer-perf-test.sh`가 만든 합성 JSON 텍스트(2.4MB)였습니다. 이번엔 다른 저장소([`prometheus-and-thanos`](../../prometheus-and-thanos))의 `go-agent`가 실제로 수집하는 CPU/메모리/네트워크 메트릭을 Prometheus remote_write 와이어 포맷(protobuf)으로 인코딩한 진짜 payload로 다시 측정했습니다. go-agent에 Kafka producer 경로(`kafkasink.go`, `SEND_TARGET=kafka` / `both`, `KAFKA_COMPRESSION`)를 추가해서, 1초 간격으로 수집한 스냅샷(553B/개)을 코덱별 토픽에 90초씩 흘려보냈습니다.

**배치가 실제로 걸리게 만드는 게 핵심이었다**: `kafka-go`의 `Writer.WriteMessages`는 기본적으로 동기 호출이라, 매초 호출하는 코드가 배치 flush(`BatchTimeout=5s`)가 끝날 때까지 블로킹됩니다. 그러면 다음 tick이 그사이 버려져서 배치당 메시지가 사실상 1개로 찌그러지고, "배치를 압축한다"는 효과 자체가 사라집니다(첫 시도에서 90번 보냈는데 실제 레코드는 17~19개만 커밋된 걸 보고 발견). `Writer.Async = true` + `Completion` 콜백으로 고쳐서, 스냅샷 5~6개가 한 배치로 묶여 실제로 같이 압축되도록 만들었습니다.

**결과 (RF=2, 코덱별 90초/약 90레코드, 총 디스크 사용량 기준)**

| 코덱 | 레코드 수 | 총 디스크 사용량 | 레코드당 평균 | `none` 대비 |
|---|---|---|---|---|
| none | 90 | 51,630 B | 573.7 B | 기준 |
| **zstd** | 91 | **8,538 B** | **93.8 B** | **16.3%** |
| gzip | 89 | 9,134 B | 102.6 B | 17.9% |
| lz4 | 87 | 9,721 B | 111.7 B | 19.5% |
| snappy | 88 | 10,576 B | 120.2 B | 20.9% |

`kafka-dump-log.sh`로 실제 배치 구조를 확인하면, 레코드 5~6개(원본 합계 ~2.8~3.3KB)가 한 `RecordBatch`로 묶여 500~560바이트로 압축된 걸 볼 수 있습니다:

```
baseOffset: 0 lastOffset: 4 count: 5 ... size: 498 magic: 2 compresscodec: zstd ...
baseOffset: 5 lastOffset: 10 count: 6 ... size: 546 magic: 2 compresscodec: zstd ...
```

**합성 데이터 벤치마크와 다른 점**: 위쪽 표(JSON 로그, 2.4MB)에서는 `none` 대비 압축률이 22~35%였는데, 실제 메트릭 payload에서는 16~21%로 훨씬 좋습니다. go-agent 메트릭은 배치 안에서 라벨(`job`, `instance`, `__name__`)이 거의 그대로 반복되고 숫자 값만 조금씩 바뀌는 구조라, 압축기 입장에서 반복 패턴이 더 극단적이기 때문입니다. 코덱 순위(`zstd` > `gzip` > `lz4` > `snappy`)는 동일합니다.

**"remote_write도 snappy인데 Kafka도 snappy로 통일하면 유리하지 않나?"** → 아닙니다.
- remote_write의 snappy는 프로토콜이 강제하는 값이지 압축률 때문에 고른 게 아닙니다(요청 1건씩 압축, 배치 없음 — 지연시간 최소화가 목적).
- Kafka는 배치를 통째로 압축해서 디스크에 RF번 복제되어 오래 남고 컨슈머가 반복 fetch하므로, 압축률이 곧 저장·네트워크 비용입니다.
- 실측상 같은 배치에서 zstd가 snappy보다 22% 더 작습니다(93.8B vs 120.2B). 이 규모의 트래픽(초당 1건 미만)에서는 CPU 차이가 무시할 수준이라 zstd를 고르지 않을 이유가 없습니다.
- snappy 통일이 유리한 경우는 순수히 "운영 단순성"(라이브러리/버전 통일, 구형 컨슈머 호환) 뿐이며, 이 스택에는 해당하지 않습니다.

**재현**: `prometheus-and-thanos/03-thanos-receive/go-agent`에서 `SEND_TARGET=kafka`, `KAFKA_BROKERS=kafka-1:29092,kafka-2:29092`, `KAFKA_TOPIC=go-agent-bench-<codec>`, `KAFKA_COMPRESSION=<codec>`, `COLLECT_INTERVAL=1s` 환경변수로 컨테이너를 90초 정도 띄운 뒤, 위와 동일하게 `kafka-log-dirs.sh --describe`로 비교.

**환경 참고**: 이 실험 도중 3.과 4.에서 다룬 Colima VM(4GiB) OOM 문제가 실시간으로 재현됐습니다 — kafka-2/3가 켜지자마자 kafka-1이 죽는 두더지잡기 상태였고, 같은 세션에서 쓰던 `03-thanos-receive`의 Thanos/Grafana 스택을 일시적으로 내려서 메모리를 확보한 뒤에야 3노드가 동시에 안정적으로 떠 있었습니다. 공유 개발 환경에서 Kafka 3노드 + 다른 무거운 스택을 동시에 돌릴 땐 이 여유가 필요하다는 뜻입니다.

## 실전 사례 2: remote_write에도 배치를 넣어서 "같은 배치, 다른 코덱"으로 재측정

위 실전 사례 1은 1틱(1스냅샷)마다 별도 메시지/요청을 보냈습니다. 그런데 remote_write의 `WriteRequest`는 원래 한 시계열(labels 동일)에 샘플을 여러 개 담을 수 있게 설계돼 있어서(`TimeSeries { labels, repeated Sample }`), go-agent가 매 틱 보낼 필요 없이 `BATCH_SIZE`틱만큼 모았다가 라벨을 한 번만 싣고 값-타임스탬프만 쌓아서 보내도록 고쳤습니다(`BATCH_SIZE` 환경변수, 기본값 1=기존 동작 유지). 이러면 remote_write와 Kafka 양쪽에 **완전히 동일한 배치 payload**를 흘려보내고 코덱만 바꿔서 비교할 수 있습니다.

**설정**: `COLLECT_INTERVAL=1s`, `BATCH_SIZE=5` → 5개 스냅샷(25 샘플)을 라벨 반복 없이 하나의 `WriteRequest`로 인코딩 → raw 918B. 이 동일한 918B payload를 remote_write(snappy, 스펙 강제)와 Kafka(코덱별)로 각각 90초씩 흘려보냄.

| 경로 | 코덱 | 레코드/배치 수 | 총 바이트 | 배치당 평균 | raw(918B) 대비 |
|---|---|---|---|---|---|
| Kafka | none | 17 | 16,376 B | 963.3 B | 104.9%(레코드 오버헤드) |
| Kafka | **gzip** | 18 | **6,671 B** | **370.6 B** | **40.4%** |
| Kafka | zstd | 17 | 6,514 B | 383.2 B | 41.7% |
| Kafka | snappy | 18 | 7,241 B | 402.3 B | 43.8% |
| Kafka | lz4 | 17 | 6,863 B | 403.7 B | 44.0% |
| **remote_write** | snappy(강제) | 6개 표본 평균 | - | **~429.3 B** | **~46.8%** |

(remote_write 행은 로그에서 뽑은 표본 6개의 평균이라 Kafka 쪽처럼 브로커 전체 합산은 아니지만, 같은 918B payload·같은 snappy 계열이라 자릿수 비교엔 충분합니다.)

**두 가지 발견**:
1. **이 배치 크기(918B, 5샘플)에서는 순위가 살짝 바뀝니다.** 실전 사례 1(배치 없음, 개별 553B 압축)에서는 `zstd > gzip > lz4 > snappy` 순이었는데, 여기선 `gzip(40.4%) > zstd(41.7%) > snappy(43.8%) ≈ lz4(44.0%)`로 gzip이 근소하게 zstd를 앞섭니다. 배치가 작을수록 압축 사전(dictionary)을 세팅하는 오버헤드 비중이 커져서 zstd의 장점(레벨 조절 가능한 고압축)이 덜 드러나고, gzip의 단순 압축이 이 크기대에서는 살짝 유리해집니다. **배치 크기가 커질수록 zstd가 다시 앞서는 경향**은 실전 사례 1(더 큰 배치)과 앞쪽 합성 벤치마크(2.4MB)에서 이미 확인됨 — 결론은 "무조건 zstd"가 아니라 "배치가 클수록 zstd, 아주 작은 배치는 gzip도 경쟁력 있음"입니다.
2. **remote_write의 snappy(46.8%)가 Kafka의 snappy 구현(43.8%)보다 약간 더 큽니다.** 완전히 동일한 원본 바이트인데도 차이가 나는 건, remote_write는 raw snappy block 포맷을 그대로 쓰고 Kafka의 snappy 코덱은 xerial 프레이밍(청크 단위 길이 헤더)을 추가로 얹기 때문입니다 — 같은 알고리즘이라도 프레이밍 오버헤드가 다르면 최종 바이트 수가 달라집니다.

**결론**: "remote_write가 snappy를 쓰니까 Kafka도 snappy로 맞추자"는 이득이 없는 정도가 아니라, 오히려 **같은 데이터를 Kafka의 gzip/zstd로 보내는 게 remote_write의 snappy보다도 더 작습니다**(40~42% vs 46.8%). remote_write 쪽 압축 효율을 올리고 싶으면 코덱을 바꿀 수 없으니(스펙 강제) 답은 하나, **배치 크기를 키우는 것**뿐입니다.

## 토픽/프로듀서마다 압축 코덱을 다르게 가져갈 수 있는가

가능하고, 두 레벨로 나뉩니다.

**1) 프로듀서 레벨(권장, 지금까지 한 방식)** — `compression.type=producer`(브로커/토픽 기본값)에서는 브로커가 재압축하지 않고 프로듀서가 압축한 그대로 저장합니다. 이 실험에서 `go-agent-bench-none/gzip/snappy/lz4/zstd`처럼 **토픽별로** 다른 프로듀서 설정을 쓴 게 이 경우입니다. CPU 비용 추가 없음.

**2) 토픽 레벨 강제(거버넌스 목적)** — 토픽 자체에 특정 코덱을 지정하면, 브로커가 프로듀서가 뭘 보냈든 그 코덱으로 재압축해서 저장합니다:
```bash
kafka-configs.sh --alter --entity-type topics --entity-name <topic> \
  --add-config compression.type=zstd \
  --bootstrap-server kafka-1:29092
```
여러 팀/언어 클라이언트가 같은 토픽에 쓸 때 프로듀서 설정에 의존하지 않고 저장 정책을 강제하고 싶을 때 씀. 브로커가 압축 해제 후 재압축하는 CPU 비용이 추가로 듦.

**"그럼 `none`으로 안 정하면 앱이 직접 압축/해제해야 하는 거 아냐?" → 아닙니다.** Kafka의 RecordBatch 포맷은 **자기 자신이 어떤 코덱으로 압축됐는지 배치 헤더(`attributes` 필드)에 직접 담고 있습니다.** 그래서 `compression.type=producer`(기본값)일 때, 같은 토픽에 프로듀서 A는 zstd로, 프로듀서 B는 snappy로, 프로듀서 C는 압축을 아예 안 하고 보내도 컨슈머 클라이언트 라이브러리가 **배치마다 자기 헤더를 읽고 알아서** 압축을 해제합니다 — 애플리케이션은 코덱을 신경 쓸 필요가 전혀 없습니다. 즉 "토픽 안에서 코덱이 섞이는 것"과 "`none`인지 아닌지"는 무관합니다.

진짜 `none`(Kafka의 압축 기능 자체를 끔 — 압축 안 하거나, 토픽에 `compression.type=none`을 강제)일 때만 얘기가 달라집니다. 그때는 Kafka가 코덱 정보를 아예 기록하지 않으니, 압축을 원하면 애플리케이션이 값(value)을 직접 압축해서 넣고 컨슈머도 직접 해제해야 하며, **어떤 코덱을 썼는지 자체도 애플리케이션이 어딘가(메시지 헤더, 매직 바이트, 사전 합의)에 기록**해야 컨슈머가 풀 수 있습니다 — Kafka 네이티브 압축보다 오히려 더 번거로운 방식이라 실무에서 잘 안 씁니다.

정리:
| `compression.type` | 코덱 결정 | 프로듀서마다 다른 코덱 | 앱이 압축 관여? |
|---|---|---|---|
| `producer`(기본) | 프로듀서가 정함, 배치 헤더에 자동 기록 | 자유롭게 섞임(같은 토픽 안에서도) | 안 함 — Kafka가 배치 헤더로 자동 인식/해제 |
| `<특정 코덱>`(토픽 강제) | 브로커가 강제 재압축 | 안 됨(전부 통일) | 안 함 |
| `none`(압축 기능 자체를 끔) | 없음 | 앱이 직접 하면 자유(대신 관리 부담) | 함 — 코덱 선택·기록·해제 전부 앱 책임 |

## 실전 사례 3: 배치 크기에 따른 gzip vs zstd 성능 차이

`BATCH_SIZE`(go-agent가 한 `WriteRequest`에 담는 샘플 수)를 1/5/20/50/100으로 바꿔가며 `COLLECT_INTERVAL=100ms`로 60초씩 gzip·zstd 두 코덱만 비교했습니다. `BATCH_SIZE`가 커질수록 앱이 한 번에 보내는 payload(raw)도 커집니다: 1→553B, 5→918B, 20→2,268B, 50→4,968B, 100→9,468B.

**1차 결과(압축률 = 압축 후 바이트 / raw 바이트)만 보면 오히려 배치가 작을수록 좋아 보입니다:**

| BATCH_SIZE | raw | gzip 압축률 | zstd 압축률 |
|---|---|---|---|
| 1 | 553 B | 7.3% | 6.4% |
| 5 | 918 B | 17.9% | 15.7% |
| 20 | 2,268 B | 25.6% | 22.2% |
| 50 | 4,968 B | 27.1% | 21.7% |
| 100 | 9,468 B | 27.2% | 21.1% |

**이건 함정입니다.** `kafka-dump-log.sh`로 실제 압축 단위(RecordBatch)를 까보면, `BATCH_SIZE=1`처럼 전송이 잦은 설정에서는 kafka-go 프로듀서 자체의 배치 타임아웃(`BatchTimeout=5s`)이 그사이 도착한 **개별 메시지 51개를 한 번에 묶어서** 압축합니다(`count: 51, size: 1916`) — 그런데 이 51개는 각각 라벨(`job`,`instance`,`__name__`)을 통째로 반복해서 들고 있는 완전히 독립된 protobuf 블롭이라, 압축기 입장에선 "같은 문자열이 51번 반복"이라는 극단적으로 유리한 데이터를 만나는 겁니다. 반대로 `BATCH_SIZE=100`은 전송 간격(10초)이 kafka-go의 5초 타임아웃보다 길어서 **메시지 하나(=RecordBatch 하나)에 압축이 끝나고**(`count: 1`), 애초에 `BATCH_SIZE`가 라벨을 한 번만 실어 반복을 없앤 상태라 압축기가 찾을 반복 패턴이 적습니다. 즉 압축률 숫자는 "코덱 성능"이 아니라 "라벨을 몇 번 반복해서 압축기에 떠먹였는가"를 재고 있었던 겁니다.

**그래서 진짜 봐야 할 건 압축률이 아니라 샘플 1개당 실제 저장 바이트(절대값)입니다:**

| BATCH_SIZE | gzip (B/샘플) | zstd (B/샘플) | zstd가 gzip보다 작은 정도 |
|---|---|---|---|
| 1 | 40.21 | 35.51 | 11.7% |
| 5 | 32.93 | 28.74 | 12.7% |
| 20 | 28.99 | 25.22 | 13.0% |
| 50 | 26.97 | 21.59 | 20.0% |
| **100** | **25.78** | **19.97** | **22.5%** |

이렇게 정규화하면 그림이 완전히 뒤집힙니다: **배치가 커질수록(라벨 반복을 줄일수록) 샘플당 저장 비용은 두 코덱 다 꾸준히 줄어들고**(gzip 40.21→25.78B, zstd 35.51→19.97B), **zstd가 gzip을 앞서는 격차도 배치가 커질수록 벌어집니다**(11.7% → 22.5%). zstd의 큰 압축 윈도/딕셔너리가 입력이 커질수록 더 잘 활용되는 반면, gzip은 상대적으로 작은 윈도(32KB) 때문에 큰 입력에서 이득이 덜 커지는 것으로 보입니다.

**결론**:
1. **"압축률(%)"만 보고 배치 전략을 판단하면 안 됩니다.** 압축률은 압축 *대상*에 이미 얼마나 많은 반복(라벨 중복 등)이 들어있는지에 크게 좌우되므로, 서로 다른 배치 전략을 비교할 땐 반드시 절대 바이트(B/샘플, B/레코드)로 정규화해서 봐야 합니다.
2. **배치 크기를 키우는 것 자체가 최선의 최적화입니다** — 코덱을 아무리 잘 골라도 라벨을 계속 반복해서 보내는 것(작은 배치)보다, 라벨을 한 번만 싣고 값을 쌓아 보내는 것(큰 배치)이 압축 전 원본 크기부터 줄여주기 때문에 절대적으로 유리합니다.
3. **zstd는 배치가 클수록 gzip 대비 유리해집니다.** 아주 작은 배치(라벨이 반복돼서 이미 극단적으로 압축이 잘 되는 상황)에서는 둘 차이가 크지 않지만(11.7%), 배치가 커질수록(라벨 중복 없이 순수 데이터 밀도가 높아질수록) zstd의 우위가 뚜렷해집니다(22.5%).

**재현**: `go-agent`에 `SEND_TARGET=kafka`, `BATCH_SIZE=<1|5|20|50|100>`, `COLLECT_INTERVAL=100ms`, `KAFKA_COMPRESSION=<gzip|zstd>`로 60초씩 실행 → `kafka-get-offsets.sh`로 레코드(=전송 성공한 WriteRequest) 수, `kafka-log-dirs.sh --describe`로 토픽 총 바이트 확인 → `총 바이트 / (레코드 수 × BATCH_SIZE)` = 샘플당 바이트. 실제 압축 단위를 확인하려면 `kafka-dump-log.sh --files <log파일>`로 각 RecordBatch의 `count`(묶인 레코드 수)를 직접 봐야 합니다.
