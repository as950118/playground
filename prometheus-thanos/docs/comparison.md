# 비교 결과 기록

각 스택(`01-single-prometheus`, `02-thanos-sidecar-3x`, `03-thanos-receive`)을 하나씩 실행하며 아래 표를 채워보세요. 측정 방법은 각 스택 README와 `common/scripts/`를 참고하세요.

전제: 총 시리즈 15,000개, scrape_interval 15s(≈ 1,000 samples/sec)로 세 스택 모두 동일하게 맞춰져 있습니다. (10M+ req/day ÷ 86,400s ≈ 116 req/s 규모의 서비스가 만들어낼 법한 카디널리티로 역산한 값 — 최상위 README 참고)

## 비교표

| 항목 | 01. 단일 Prometheus | 02. Thanos Sidecar 3x | 03. Thanos Receive |
|---|---|---|---|
| 데이터 수집 경로 | scrape → 로컬 TSDB | scrape → 로컬 TSDB → sidecar 업로드 | scrape → remote_write → receive |
| 저장 위치 | 로컬 디스크만 | 로컬(단기) + 오브젝트 스토리지(장기) | 오브젝트 스토리지 위주 (receive 로컬은 단기 버퍼) |
| 로컬에서 직접 PromQL 가능? | 가능 | 가능 (각 shard Prometheus) | 불가능 (agent 모드) |
| 글로벌 뷰 | 없음 | Thanos Query로 있음 | Thanos Query로 있음 |
| 컴포넌트 1개 장애 시 | 전체 조회 중단 | shard 1개 kill해도 Thanos Query가 나머지 2개+Store로 계속 응답 (부분 데이터 유실은 있음) | receive 1개 kill해도 Query는 응답 유지. 단, RF=1이라 그 replica가 맡은 해시 구간의 시리즈는 이론상 조회 불가 — 실습에서는 warning(`rpc ... i/o timeout`)만 뜨고 결과 자체는 살아있는 다른 replica 데이터로 채워짐 |
| retention(6h) 초과 데이터 조회 | 불가 | 가능 (thanos-store가 오브젝트 스토리지 블록을 서빙) — 단, 실습 환경에서 6h를 실제로 넘겨서 확인하려면 장시간 실행 필요 | 가능 (구조상 동일, thanos-store 경유) — 마찬가지로 장시간 실행 확인 필요 |
| 쿼리 응답시간 (query_bench.sh, `up`, 1h range) | 0.0036s (303 bytes) | 0.0055s (14,103 bytes, 15,000 series 규모) | 0.0023s (1,218 bytes, 기동 직후라 데이터量 적음) |
| 디스크 사용량 (disk_usage.sh) | prometheus 528K (기동 ~35초 시점) | prometheus shard 합계 ~36.6M (11시간 경과) + MinIO 버킷 75M | prometheus-agent 합계 ~700K, thanos-receive 합계 ~1.5M (기동 직후라 매우 작음) |
| 오브젝트 스토리지에 블록 업로드까지 걸린 시간 | 해당없음 | 실측 완료: 최소 2시간(block duration) + sidecar 감지 지연. **중간에 실제 버그 발견** — sidecar가 non-root(uid 1001)로 떠서 Prometheus 볼륨(nobody 소유)에 thanos.shipper.json을 못 써서 업로드가 전부 permission denied로 실패하고 있었음. `user: "0:0"` 추가로 수정, 3개 shard 블록 모두 업로드 확인(각 ~4.7MiB) | 미확인 — 아직 2시간 이상 실행하지 않음 (02와 달리 receive는 처음부터 root로 떠서 같은 버그는 없을 것으로 예상되지만 실제 확인은 못함) |
| 구성 요소 개수 (docker compose ps 기준) | 3 | 15 | 15 |
| 운영 복잡도 체감 (정성적) | 매우 단순, 장애점 1개 | 컴포넌트는 많지만(15개) 각자 역할이 명확히 분리됨(수집/저장/조회). sidecar가 로컬 볼륨 권한 이슈처럼 "겉보기엔 되는데 조용히 실패"하는 게 가장 위험한 지점 | 02와 컴포넌트 수는 같지만 Prometheus가 로컬 TSDB를 안 가지므로 엣지 노드 자체의 상태가 훨씬 가벼움(디스크/보존기간 관리 불필요). 대신 hashring 설정과 RF에 대한 이해가 필수 |

## 메모

- **sidecar 볼륨 권한 버그**: 02번 스택에서 몇 시간 뒤 실제 블록이 컴팩션됐는데도 오브젝트 스토리지에 아무것도 안 올라가고 있었음. sidecar 컨테이너가 Prometheus 소유(nobody:nobody)의 `/prometheus` 볼륨에 `thanos.shipper.json`을 쓸 권한이 없어서 업로드가 조용히 계속 실패하던 것. 초기 검증 때는 블록이 아직 안 만들어져서 못 잡았던 문제 — "짧게 띄워서 확인"만으로는 이런 클래스의 버그를 못 잡는다는 교훈.
- **"Thanos만으로 운영 가능한가?"에 대한 답**: 엄밀히는 아니다. Thanos는 스크레이핑 기능이 전혀 없다 (Receive는 remote_write로 push되는 걸 받기만 함). 03번 스택도 Prometheus를 Agent 모드로 여전히 쓰고 있으며, 이건 "로컬 TSDB/보존/컴팩션 책임을 Thanos에 완전히 이관하고, Prometheus는 스크레이퍼로만 축소"한 구성이다. Prometheus 자리를 OpenTelemetry Collector나 vmagent, Grafana Alloy 같은 다른 remote_write 소스로 바꿔도 성립하지만, "메트릭을 실제로 긁어오는 무언가"는 항상 필요하다. 대규모 운영에서 실제로 쓰이는 패턴(예: 수천 개 엣지 노드마다 무거운 Prometheus TSDB를 두지 않고 싶을 때)이지만 "Prometheus 없는 Thanos"가 아니라 "Prometheus가 얇아진 Thanos"가 더 정확한 표현.
- **Prometheus 자체 구현 없이 remote_write만으로 데이터 주입 가능함을 실증**: protobuf(WriteRequest/TimeSeries/Label/Sample)를 손으로 인코딩하고 snappy(raw block, literal-only 인코딩)로 압축해서 `http://thanos-receive-1:19291/api/v1/receive`에 직접 POST → `http_code=200`, Thanos Query로 즉시 조회됨. 즉 remote_write 스펙(protobuf 스키마 + snappy 압축 + 정해진 헤더)만 지키면 Kafka consumer든 커스텀 Go agent든 무엇이든 Thanos Receive의 데이터 소스가 될 수 있음. (주의: 라벨은 이름 기준 오름차순 정렬이 안 되어 있으면 `409 out of order labels`로 거부됨)
- **최근 데이터 vs 오래된 데이터 쿼리 속도 실측 (02번 스택, retention 6h 지난 17시간 전 블록 기준)**:
  - 로컬 Prometheus 직접 조회(17h 전, retention 밖) → 2ms, 결과 없음(이미 로컬에서 삭제됨)
  - Thanos Query로 같은 17h 전 조회, **최초(콜드)** → 11.4ms (Store Gateway가 MinIO에서 index/chunk를 새로 가져옴)
  - 같은 조회 **재실행(캐시 warm)** → 3.3~4.0ms로 수렴
  - 최근 5분 조회(로컬 sidecar 경유) → 2.3~4.0ms
  - → 결론: 최근 데이터는 Prometheus/Thanos 둘 다 비슷하게 빠름. 로컬 retention을 넘어간 데이터는 Thanos만 조회 가능하지만 콜드 액세스에 확실한 페널티(이 소규모 환경에서도 ~3~5배)가 있고, 캐시로 완화됨. 실제 프로덕션(진짜 S3, 더 큰 데이터량)에서는 이 격차가 훨씬 커짐 → Store Gateway 앞에 캐시(memcached 등)나 Query Frontend가 사실상 필수.

## 실전 적용 시나리오: 수천 대 규모 인프라 메트릭 수집

**사용자 환경**:
- 수집 대상: 단일 노드가 아니라 관리 중인 **수천 개 이상의 VM/노드**
- 수집 방식: **자체 개발 Go agent**로 각 노드에서 수집 → **Kafka**로 전송 → **OTel**(Collector로 추정) 경유
- 수집 주기: 약 10초 간격
- 수집 메트릭: CPU, 메모리, 네트워크 등 인프라 사용량 (노드당 다수 시리즈 × 수천 노드 → 최소 수만~수십만 활성 시리즈 규모로 이 스터디의 15,000개보다 한 자릿수 이상 큼)

**선택: 옵션 B — Thanos Receive 단독 운영(03번 구조) + 캐시(memcached/redis)**. Prometheus+Sidecar(02번 구조)는 채택하지 않음.

**이유**:
1. **스크레이핑 모델 자체가 안 맞음.** 이미 push 기반(Go agent → Kafka → OTel) 파이프라인이 있는데 Prometheus를 넣으려면 오히려 각 노드에 exporter를 세우고 Prometheus가 다시 pull하게 만드는 역행이 됨. Prometheus의 핵심 가치(서비스 디스커버리, relabel, 스크레이프 스케줄링)가 이 구조에서는 아무 의미가 없음.
2. **이미 검증된 경로.** remote_write 스펙(protobuf+snappy)만 지키면 무엇이든 데이터 소스가 될 수 있다는 걸 위에서 직접 증명함 — OTel Collector의 `prometheusremotewrite` exporter로 Kafka에서 소비한 데이터를 그대로 Thanos Receive에 밀어넣으면 됨.
3. **노드 수천 개 = per-node Prometheus 운영 비용이 감당 불가.** 노드마다 설정 배포·디스크 프로비저닝·업그레이드·보존기간 관리를 해야 한다면, 이미 확보한 경량 Go agent의 이점이 사라짐. 03번 스택에서 실측한 대로 agent 쪽 로컬 디스크 사용량은 사실상 0(700KB 수준)이라 수천 대로 늘려도 엣지 노드 부담이 거의 없음.
4. **카디널리티 규모상 Receive 수평 확장이 필수.** 이 스터디의 3-shard hashring과 동일한 원리로, 실제로는 receive 노드 수를 늘려서 해시링을 더 잘게 쪼개야 함.
5. **인프라 모니터링은 "최근 데이터" 조회가 압도적으로 많음(알림/실시간 대시보드)** → 위에서 확인한 대로 이 트래픽이 캐시 상태에 종속되는 게 가장 큰 리스크. **완화책: Thanos Receive의 로컬 보존 기간(블록 업로드 전 유지 기간)을 24~48h 정도로 넉넉히 설정**해서, 운영 대시보드/알림이 보는 "핫 윈도우"는 로컬에서 서빙되게 하고 진짜 히스토리성 조회만 Store Gateway+cache 경로를 타게 만들 것.
6. **RF(replication factor) 상향.** 인프라 모니터링은 페이징으로 이어지는 경우가 많으므로, 이 스터디의 RF=1이 아니라 RF≥2~3으로 설정해 receive 노드 1대 장애가 알림 유실로 이어지지 않게 할 것.
7. **memcached/redis는 선택적 최적화가 아니라 필수 프로덕션 컴포넌트로 취급.** 자체 모니터링·이중화 대상에 포함시키고, "캐시가 죽어도 서비스에 영향 없음"이 아니라 "캐시가 죽으면 최근 쿼리까지 느려질 수 있다"는 전제로 SLO를 잡을 것.

### 03번 스택에 실제 반영한 내용

위 권장 구성을 `03-thanos-receive/docker-compose.yml`에 실제로 적용하고 전부 재기동/검증까지 완료함:

- `--tsdb.retention=48h`: receive-1/2/3 전부에 추가 (기본값 15d보다 짧게 명시적으로 잡아서 "핫 윈도우는 로컬" 정책을 문서화)
- `--receive.replication-factor=1` → `=3`: 실측 결과 세 receive 노드의 `prometheus_tsdb_head_series`가 전부 16,572로 동일 — 정상 복제 확인
- `memcached` 서비스 추가 + `thanos-store`에 `--index-cache.config-file`(`thanos/index-cache.yml`, type: MEMCACHED)로 연결 — `thanos-store` 로그에 `msg="created index cache"` 확인, 에러 없음
- **`thanos-query`에 `--query.replica-label=receive_replica` 추가 — 이 작업 중 실제로 발견한 버그.** RF를 1→3으로 올리기만 하고 이 플래그를 안 넣었더니, Thanos Query가 복제본 3개(`receive_replica="1"/"2"/"3"`)를 서로 다른 시리즈로 인식해서 `count(up) by (job)`이 원래 3이어야 할 값이 **9로 부풀려짐**. 이 플래그를 추가하고 나서야 3으로 정상화됨. RF를 올릴 때 dedup용 replica-label 플래그를 같이 안 넣으면 글로벌 뷰의 집계 쿼리(count, sum 등)가 전부 replication factor배만큼 부풀려지는 조용한 버그가 생긴다는 교훈 — sidecar 권한 버그와 마찬가지로 "에러 없이 그냥 숫자가 틀리게 나오는" 종류라 알아채기 더 어려움.
- 재검증: `--query.replica-label` 적용 후 `receive-1`을 kill해도 `count(up) by (job)`이 **3/3 그대로 유지**(dedup으로 나머지 2개 replica가 매끄럽게 커버) — RF=1 때는 같은 테스트에서 값 자체는 안 부풀었지만 대신 fetch 실패 warning이 떴던 것과 대비됨

### Thanos Ruler 추가 — "Prometheus 없이 alerting까지 가능한가?"의 실증

앞서 "Prometheus 없이 조회는 되는데 alerting은 Thanos Ruler라는 별도 컴포넌트가 필요하다"고 정리했던 부분을 실제로 03번 스택에 추가하고 end-to-end로 검증함.

- 추가한 컴포넌트: `thanos-ruler`(rule 평가, `--query=http://thanos-query:10902`로 PromQL 평가 대상 지정, `--alertmanagers.url`로 알림 전송), `alertmanager`(데모용 `default` 리시버만 있는 최소 설정)
- 룰 파일(`03-thanos-receive/rules/demo-rules.yaml`): alerting rule 2개(`TargetDown`: `up == 0`, `AvalancheGaugeTooHigh`: `avg(avalanche_gauge_metric_mmmmm_0_0) > 80`) + recording rule 2개(`job:up:sum`, `avalanche:gauge_0:avg`)
- **실제 알림 발동까지 실증**: `avalanche-1` 컨테이너를 정지 → 45초 후 `thanos-ruler`의 `/api/v1/rules`에서 `TargetDown`이 `state: firing`으로 전환 확인 → 같은 시각 `alertmanager`의 `/api/v2/alerts`에서 동일 alert(`fingerprint`, `labels.instance=avalanche-1:9001`) 수신 확인. 즉 Prometheus 없이도 감지→평가→라우팅까지 전체 alerting 파이프라인이 완결됨.
- recording rule 결과(`job:up:sum`)는 `thanos-ruler` 자신의 API뿐 아니라 **Thanos Query(글로벌 뷰)로도 조회됨** — ruler도 다른 컴포넌트처럼 StoreAPI를 노출해서 `thanos-query`의 `--endpoint` 목록에 등록되기 때문. Prometheus의 "룰 평가 + 그 결과를 자기 TSDB에 저장" 기능이, Thanos에서는 "별도 컴포넌트가 평가 + StoreAPI로 노출"하는 방식으로 그대로 대체됨을 확인.

### 실제 Go agent 개발 — "자체 개발 agent" 시나리오를 코드로 검증

이전에 raw curl로 손으로 만든 protobuf를 직접 쏘아본 것을, 실제 시스템 지표를 읽어 주기적으로 전송하는 진짜 프로그램(`03-thanos-receive/go-agent/`)으로 재현함. Prometheus는 이 경로에 전혀 등장하지 않음.

- 수집: `gopsutil`로 CPU 사용률/메모리 사용량/네트워크 송수신 바이트를 10초 간격으로 실측 (사용자가 말한 실제 운영 주기와 동일)
- 인코딩: `prometheus/prometheus` 전체를 의존성으로 끌어오지 않고, WriteRequest/TimeSeries/Label/Sample protobuf를 직접 구현(`remotewrite.go`) — 라벨은 이름 오름차순 정렬 필수(안 하면 `409 out of order labels`, 앞서 raw curl 데모에서 겪었던 것과 동일한 제약)
- 압축: 이번엔 손으로 만든 snappy 인코더가 아니라 `github.com/golang/snappy`(레퍼런스 구현)를 사용 — 앞서 raw curl 데모에서 `cramjam`의 압축 결과가 Thanos의 s2 디코더와 호환 안 됐던 문제(`s2: corrupt input`)를 겪었던 것과 달리, 실제 Go 프로덕션에서 쓸 법한 정식 라이브러리라 별다른 삽질 없이 바로 성공함
- 실측 확인: `docker logs go-agent`에 10초마다 `sent: cpu=9.9% mem=55.6% ...` 로그, 에러 없음. `curl 'http://localhost:10904/api/v1/query?query={job="go-custom-agent"}'`로 `node_cpu_usage_percent`, `node_memory_used_bytes` 등이 실제 값으로 조회됨. counter 메트릭(`node_network_receive_bytes_total`)에 `rate()`도 정상 동작(≈47B/s) 확인.
- 결론: "Kafka/OTel 대신 간단한 Go agent 하나로도 이 아키텍처가 성립하는가?"에 대한 답은 명확히 예 — remote_write 스펙(protobuf 인코딩 + 라벨 정렬 + snappy 압축 + 헤더 3개)만 지키면 되고, 실제로 이를 만족하는 최소 구현은 의존성 2개(`gopsutil`, `golang/snappy`)와 200줄 미만의 Go 코드로 충분함.
