# Prometheus & Thanos 비교 스터디

"일 1000만 건 이상 요청을 처리하는 서비스"를 모니터링한다고 가정하고, 아래 3가지 구성을 직접 띄워보며 차이를 비교하는 스터디용 레포입니다.

1. **[01-single-prometheus](./01-single-prometheus)** — Thanos 없이 Prometheus 단독 (baseline)
2. **[02-thanos-sidecar-3x](./02-thanos-sidecar-3x)** — Prometheus를 3개 샤드로 나누고 각각에 Thanos Sidecar를 붙여 오브젝트 스토리지 + 글로벌 뷰 구성
3. **[03-thanos-receive](./03-thanos-receive)** — 엣지 Prometheus는 Agent 모드로 스크랩만 하고, 저장/조회는 Thanos Receive/Query가 전담 (Thanos로만 구성)

세 스택 모두 **총 시리즈 15,000개, scrape_interval 15초(≈1,000 samples/sec)** 로 동일하게 맞춰서, 순수하게 아키텍처 차이만 비교할 수 있게 했습니다.

## 왜 이 숫자인가 (부하 산정 근거)

- 10,000,000 req/day ÷ 86,400s ≈ **116 req/s 평균** (피크는 3~5배인 350~580 req/s로 가정)
- 이 정도 트래픽을 가진 실제 서비스는 라벨 조합(엔드포인트, 상태코드, 인스턴스 등)에 따라 활성 시리즈가 수천~수만 개인 경우가 흔함 → 목표를 **총 15,000 시리즈**로 설정
- scrape_interval 15초 기준 샘플 수집률 = 15,000 / 15 ≈ **1,000 samples/sec** — 노트북에서 무리 없이 돌아가면서도 "고부하"를 체감할 수 있는 수준
- 실제 요청을 흉내내는 대신 [avalanche](https://github.com/prometheus-community/avalanche)로 시리즈 수·라벨 카디널리티를 직접 제어해서 합성 부하를 생성합니다 (재현성이 더 높고, 커스텀 앱 코드가 필요 없음)
- avalanche의 시리즈 churn(`--series-interval`)은 `0`(비활성화)으로 고정했습니다 — 기본값(60s)을 켜두면 라벨 조합이 계속 새로 생성되며 시리즈 수가 무한히 늘어나 세 스택을 같은 조건에서 비교하기 어려워집니다. 값 자체는 `--value-interval=10`으로 계속 바뀌게 해서 "살아있는" 메트릭처럼 보이게 합니다.

## 실행 방법

세 스택은 **독립된 docker-compose 프로젝트**이며, 포트가 동일하게 겹치도록 맞춰뒀기 때문에 **한 번에 하나씩만** 실행합니다.

```bash
cd 01-single-prometheus   # 또는 02-thanos-sidecar-3x, 03-thanos-receive
docker compose up -d
# ... 실습 & docs/comparison.md 기록 ...
docker compose down -v
cd ..
```

각 스택 폴더의 README에 관찰 포인트와 실습 스크립트 사용법이 정리되어 있습니다.

## 공용 도구

- `common/grafana/dashboards/comparison.json` — 세 스택에 동일하게 프로비저닝되는 비교 대시보드 (Targets Up, Head Series Count, Ingestion Rate, Scraped Samples by Job, TSDB Storage Bytes, Scrape Duration)
- `common/scripts/query_bench.sh <base-url> [query] [range-seconds]` — range query 응답시간 측정
- `common/scripts/disk_usage.sh <stack-dir>` — TSDB/오브젝트 스토리지 디스크 사용량 확인
- `common/scripts/kill_and_check.sh <stack-dir> <service> <base-url> [query]` — 컴포넌트 하나를 정지시키고 조회 API가 계속 응답하는지 확인 (장애 내성 비교)

## 비교 결과 기록

실습하면서 [docs/comparison.md](./docs/comparison.md)의 표를 채워나가세요.
