# 03. Thanos Receive 기반 (Thanos로만 구성)

엣지의 Prometheus는 **Agent 모드**(`--agent`)로만 동작합니다 — 로컬에 조회 가능한 TSDB가 없고, 스크랩한 데이터를 remote_write로 즉시 Thanos Receive에 떠넘깁니다. 저장(TSDB), 오브젝트 스토리지 업로드, 글로벌 조회는 전부 Thanos 컴포넌트(Receive/Store/Compact/Query)가 전담합니다 — 02번과 달리 "실제 저장소 역할을 하는 Prometheus"가 아예 없습니다.

02번과 총 시리즈 수(15,000개, 5,000개 x 3)는 동일하게 맞췄습니다.

## 구성

| 컴포넌트 | 역할 |
|---|---|
| `avalanche-1/2/3` | 샤드당 5,000 시리즈 |
| `prometheus-agent-1/2/3` | 스크랩 전용. 로컬 저장 없이 `thanos-receive-N`으로 remote_write |
| `thanos-receive-1/2/3` | remote_write 수신 + 로컬 TSDB 적재 + MinIO 업로드. `hashring.json`으로 시리즈를 노드별로 분배 (`replication-factor=3`, `tsdb.retention=48h`) |
| `thanos-store` | MinIO에 업로드된 과거 블록 조회. memcached 기반 index-cache 연결 |
| `thanos-compact` | 블록 압축/다운샘플링 |
| `thanos-query` | receive 3개 + store gateway + ruler를 합쳐 글로벌 조회 (`--query.replica-label=receive_replica`로 복제본 dedup) |
| `thanos-ruler` | `rules/demo-rules.yaml`의 alerting/recording rule을 Thanos Query에 대고 주기 평가, firing 알림을 Alertmanager로 전송. Prometheus의 룰 평가 기능을 대체하는 컴포넌트 |
| `alertmanager` | Ruler가 보낸 알림을 라우팅/수신 (데모용 `default` 리시버, 실제 알림 채널 연동은 없음) |
| `memcached` | Store Gateway의 index-cache 백엔드 |
| `minio` | S3 호환 오브젝트 스토리지 |
| `grafana` | Thanos Query를 데이터소스로 사용 |

> **운영 튜닝 반영됨**: 수천 대 규모 노드에서 자체 개발 agent(Kafka/OTel 등)로 데이터를 밀어넣는 시나리오를 가정해 `replication-factor=1→3`, `tsdb.retention=48h`(핫 윈도우는 로컬에서 서빙), memcached index-cache를 반영했습니다. 근거는 `../docs/comparison.md`의 "실전 적용 시나리오" 절 참고.

## 실행

```bash
docker compose up -d
```

- Thanos Query (조회 진입점): http://localhost:10904
- Prometheus agent-1 상태 페이지(쿼리 불가, 타겟 상태만): http://localhost:9090
- Thanos Receive-1 자체 지표: http://localhost:10905
- Thanos Ruler (룰/알림 상태): http://localhost:10906
- Alertmanager: http://localhost:9093
- Grafana: http://localhost:3000
- MinIO 콘솔: http://localhost:19001 (minioadmin / minioadmin)
- Avalanche shard-1 원본 메트릭: http://localhost:9091/metrics

## 관찰 포인트

1. **Prometheus agent에는 쿼리 API가 없다** — http://localhost:9090/graph 에서 PromQL을 실행하면 에러가 납니다. 데이터를 보려면 반드시 Thanos Query(10904)를 거쳐야 합니다. 이게 02번(sidecar)과 가장 큰 차이입니다: 02는 각 Prometheus에서도 최근 데이터를 직접 조회할 수 있었습니다.
2. **remote_write 경로 확인** — `docker compose logs thanos-receive-1 | grep -i "successfully"` 류로 수신 로그를 확인해보세요.
3. **receive 노드 하나가 죽으면?**
   ```bash
   ../common/scripts/kill_and_check.sh . thanos-receive-1 http://localhost:10904 'count(up) by (job)'
   ```
   replication-factor=3이라 `count(up) by (job)`이 노드 1개가 죽어도 값 그대로(3/3) 유지됩니다 — 다른 2개 replica가 매끄럽게 커버. 다만 `--query.replica-label=receive_replica`가 **반드시 같이 있어야** 합니다: 이게 없으면 Thanos Query가 복제본 3개를 서로 다른 시리즈로 착각해서 집계 쿼리 결과가 replication-factor배(3배)로 부풀려지는 조용한 버그가 생깁니다(직접 겪은 문제 — RF만 올리고 이 플래그를 빼먹었더니 `count(up)`이 9로 나왔습니다). replication-factor를 올리면 그만큼 저장 비용도 커진다는 트레이드오프는 여전히 유효합니다.
4. **오브젝트 스토리지 업로드 확인** — MinIO 콘솔에서 `thanos` 버킷에 블록이 쌓이는지 확인 (receive가 직접 업로드).
5. **쿼리 응답시간 / 디스크 사용량 비교**
   ```bash
   ../common/scripts/query_bench.sh http://localhost:10904 up
   ../common/scripts/disk_usage.sh .
   ```
6. **알림(Alerting)이 실제로 동작하는지 확인** — Prometheus 없이도 Thanos Ruler가 룰 평가를 전담합니다.
   ```bash
   # 룰 평가 상태 확인 (rule.go가 15초 간격으로 evaluate)
   curl -s http://localhost:10906/api/v1/rules | python3 -m json.tool

   # 타겟 하나를 죽여서 TargetDown 알림을 실제로 발동시켜보기
   docker compose stop avalanche-1
   sleep 45   # for: 15s + eval-interval 15s 여유를 두고 대기
   curl -s http://localhost:10906/api/v1/rules   # TargetDown이 "firing"으로 바뀜
   curl -s http://localhost:9093/api/v2/alerts   # Alertmanager에도 도착했는지 확인
   docker compose start avalanche-1
   ```
   recording rule 결과(`job:up:sum`, `avalanche:gauge_0:avg`)는 Thanos Query(10904)로도 조회됩니다 — ruler가 만든 시리즈도 다른 컴포넌트와 동일하게 StoreAPI로 글로벌 뷰에 합류하기 때문입니다.

## 정리

```bash
docker compose down -v
```
