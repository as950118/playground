# 03. Thanos Receive 기반 (Thanos로만 구성)

엣지의 Prometheus는 **Agent 모드**(`--agent`)로만 동작합니다 — 로컬에 조회 가능한 TSDB가 없고, 스크랩한 데이터를 remote_write로 즉시 Thanos Receive에 떠넘깁니다. 저장(TSDB), 오브젝트 스토리지 업로드, 글로벌 조회는 전부 Thanos 컴포넌트(Receive/Store/Compact/Query)가 전담합니다 — 02번과 달리 "실제 저장소 역할을 하는 Prometheus"가 아예 없습니다.

02번과 총 시리즈 수(15,000개, 5,000개 x 3)는 동일하게 맞췄습니다.

## 구성

| 컴포넌트 | 역할 |
|---|---|
| `avalanche-1/2/3` | 샤드당 5,000 시리즈 |
| `prometheus-agent-1/2/3` | 스크랩 전용. 로컬 저장 없이 `thanos-receive-N`으로 remote_write |
| `thanos-receive-1/2/3` | remote_write 수신 + 로컬 TSDB 적재 + MinIO 업로드. `hashring.json`으로 시리즈를 노드별로 분배 (replication-factor=1) |
| `thanos-store` | MinIO에 업로드된 과거 블록 조회 |
| `thanos-compact` | 블록 압축/다운샘플링 |
| `thanos-query` | receive 3개 + store gateway를 합쳐 글로벌 조회 |
| `minio` | S3 호환 오브젝트 스토리지 |
| `grafana` | Thanos Query를 데이터소스로 사용 |

## 실행

```bash
docker compose up -d
```

- Thanos Query (조회 진입점): http://localhost:10904
- Prometheus agent-1 상태 페이지(쿼리 불가, 타겟 상태만): http://localhost:9090
- Thanos Receive-1 자체 지표: http://localhost:10905
- Grafana: http://localhost:3000
- MinIO 콘솔: http://localhost:19001 (minioadmin / minioadmin)
- Avalanche shard-1 원본 메트릭: http://localhost:9091/metrics

## 관찰 포인트

1. **Prometheus agent에는 쿼리 API가 없다** — http://localhost:9090/graph 에서 PromQL을 실행하면 에러가 납니다. 데이터를 보려면 반드시 Thanos Query(10904)를 거쳐야 합니다. 이게 02번(sidecar)과 가장 큰 차이입니다: 02는 각 Prometheus에서도 최근 데이터를 직접 조회할 수 있었습니다.
2. **remote_write 경로 확인** — `docker compose logs thanos-receive-1 | grep -i "successfully"` 류로 수신 로그를 확인해보세요.
3. **receive 노드 하나가 죽으면?**
   ```bash
   ../common/scripts/kill_and_check.sh . thanos-receive-1 http://localhost:10904 up
   ```
   replication-factor=1이므로 해당 노드가 담당하던 시리즈는 재기동 전까지 조회가 끊깁니다 — 02번의 sidecar 장애(샤드 단위 부분 장애)와 성격은 비슷하지만, **agent 쪽엔 로컬 백업 저장소가 아예 없어서** receive가 완전히 복구되기 전까지는 그 구간 데이터를 다른 경로로 복구할 수 없습니다. (replication-factor를 2~3으로 올리면 완화되지만 저장 비용이 커집니다 — 대시보드 기록 시 이 트레이드오프를 적어보세요.)
4. **오브젝트 스토리지 업로드 확인** — MinIO 콘솔에서 `thanos` 버킷에 블록이 쌓이는지 확인 (receive가 직접 업로드).
5. **쿼리 응답시간 / 디스크 사용량 비교**
   ```bash
   ../common/scripts/query_bench.sh http://localhost:10904 up
   ../common/scripts/disk_usage.sh .
   ```

## 정리

```bash
docker compose down -v
```
