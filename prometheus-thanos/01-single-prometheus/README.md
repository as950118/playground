# 01. 단일 Prometheus (baseline)

Thanos 없이 Prometheus 단독으로 운영하는 가장 단순한 구성입니다. 나머지 두 구성과 비교할 때 기준점(baseline) 역할을 합니다.

## 구성

- `avalanche`: 목표 시리즈 15,000개(`gauge-metric-count=100 * series-count=150`)를 노출하는 합성 메트릭 소스. 10M+ req/day 서비스가 만들어낼 법한 카디널리티를 재현합니다.
- `prometheus`: 15초 간격으로 avalanche를 스크랩. **의도적으로 `retention.time=6h`로 짧게 설정**했습니다 — 로컬 디스크 보존 한계를 실습으로 체감하기 위함입니다.
- `grafana`: Prometheus를 직접 데이터소스로 붙여 공용 비교 대시보드(`common/grafana/dashboards/comparison.json`)를 표시합니다.

## 실행

```bash
docker compose up -d
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (anonymous Admin 접속 허용)
- Avalanche 원본 메트릭: http://localhost:9091/metrics

## 관찰 포인트

1. **단일 장애점** — `prometheus` 컨테이너를 멈추면 조회가 전부 끊깁니다.
   ```bash
   ../common/scripts/kill_and_check.sh . prometheus http://localhost:9090 up
   ```
2. **로컬 보존 기간 한계** — 6시간이 지난 데이터는 조회되지 않습니다. `docs/comparison.md`에 기록할 때, "장기 보관"이 필요한 순간 이 구성이 왜 한계인지 적어보세요.
3. **글로벌 뷰 없음** — 인스턴스가 여러 개로 늘어나면(샤딩) 이 구성만으로는 하나의 화면에서 합쳐볼 방법이 없습니다. (→ 02, 03에서 해결)
4. **쿼리 응답시간 / 디스크 사용량** — 비교용 스크립트로 수치를 기록해두세요.
   ```bash
   ../common/scripts/query_bench.sh http://localhost:9090 up
   ../common/scripts/disk_usage.sh .
   ```

## 정리

```bash
docker compose down -v
```
