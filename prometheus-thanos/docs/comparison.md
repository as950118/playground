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
| 컴포넌트 1개 장애 시 | 전체 조회 중단 | | |
| retention(6h) 초과 데이터 조회 | 불가 | | |
| 쿼리 응답시간 (query_bench.sh, `up`, 1h range) | | | |
| 디스크 사용량 (disk_usage.sh, 기동 후 X분 경과) | | | |
| 오브젝트 스토리지에 블록 업로드까지 걸린 시간 | 해당없음 | | |
| 구성 요소 개수 (docker compose ps 기준) | 3 | 15 | 15 |
| 운영 복잡도 체감 (정성적) | | | |

## 메모

- (스택별로 실습하며 놀랐던 점, 예상과 달랐던 점을 자유롭게 적어두세요)
