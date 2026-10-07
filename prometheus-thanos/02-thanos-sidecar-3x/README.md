# 02. Thanos Sidecar 기반 3중 Prometheus

트래픽(=시리즈)을 3개의 Prometheus 샤드로 나누고, 각 샤드에 Thanos Sidecar를 붙여 오브젝트 스토리지(MinIO)에 블록을 업로드합니다. Thanos Query가 3개 샤드 + Store Gateway를 하나의 글로벌 뷰로 합쳐 보여줍니다.

01번(단일 Prometheus)과 총 시리즈 수(15,000개)는 동일하게 맞췄습니다 — 차이는 순전히 아키텍처입니다.

## 구성

| 컴포넌트 | 역할 |
|---|---|
| `avalanche-1/2/3` | 샤드당 5,000 시리즈 (`gauge-metric-count=50 * series-count=100`) |
| `prometheus-shard-1/2/3` | 각자 자기 샤드의 avalanche만 스크랩. `external_labels.shard`로 구분. 로컬 retention은 6h로 짧게 유지 |
| `thanos-sidecar-1/2/3` | 각 Prometheus의 TSDB 블록(2h 단위)을 MinIO에 업로드 + 실시간 데이터는 StoreAPI로 직접 서빙 |
| `thanos-store` | MinIO에 이미 업로드된 과거 블록을 조회하는 Store Gateway |
| `thanos-compact` | 업로드된 블록을 압축/다운샘플링 (싱글턴으로 실행) |
| `thanos-query` | sidecar 3개 + store gateway를 합쳐 하나의 글로벌 쿼리 API 제공 |
| `minio` | S3 호환 오브젝트 스토리지 (버킷: `thanos`) |
| `grafana` | Thanos Query를 데이터소스로 사용 |

## 실행

```bash
docker compose up -d
```

- Thanos Query (조회 진입점): http://localhost:10904
- Prometheus shard-1 (참고용): http://localhost:9090
- Grafana: http://localhost:3000
- MinIO 콘솔: http://localhost:19001 (minioadmin / minioadmin)
- Avalanche shard-1 원본 메트릭: http://localhost:9091/metrics

## 관찰 포인트

1. **샤드 하나가 죽어도 전체 조회는 안 죽는다** — 01과 달리 부분 장애만 발생합니다.
   ```bash
   ../common/scripts/kill_and_check.sh . prometheus-shard-1 http://localhost:10904 up
   ```
   (shard-1의 데이터만 잠깐 비어 보이고, shard-2/3 데이터는 계속 조회됩니다.)
2. **오브젝트 스토리지 업로드 확인** — MinIO 콘솔(http://localhost:19001)에서 `thanos` 버킷에 2시간 단위 블록이 쌓이는지 확인하세요. (최초 블록은 2시간 경과 후 생성됩니다.)
3. **로컬 retention(6h)을 넘긴 데이터도 조회된다** — 01번은 6시간 뒤 데이터가 사라졌지만, 여기서는 Thanos Store Gateway가 버킷에서 계속 서빙합니다.
4. **글로벌 뷰** — Grafana 대시보드의 "Head Series Count"가 3개 샤드 합계(~15,000)로 보입니다. 개별 샤드 Prometheus에서는 각자 ~5,000개만 보입니다.
5. **쿼리 응답시간 / 디스크 사용량 비교**
   ```bash
   ../common/scripts/query_bench.sh http://localhost:10904 up
   ../common/scripts/disk_usage.sh .
   ```

## 정리

```bash
docker compose down -v
```
