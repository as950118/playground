# docs 안내

이 폴더는 루트의 `README.md`(실습 런북: 명령어를 그대로 따라 치는 가이드)에서 다루지 않은 **개념/설계 판단/트러블슈팅**을 주제별로 정리한 문서입니다.

- [01-cluster-architecture.md](01-cluster-architecture.md) — 3노드 KRaft 클러스터를 왜 이렇게 구성했는지 (리스너 분리, 이미지 선택, 쿼럼)
- [02-jbod-vs-scale-out.md](02-jbod-vs-scale-out.md) — JBOD(1브로커-N디스크) vs 브로커 확장(N브로커-N디스크) 구조 비교
- [03-compression.md](03-compression.md) — 압축 방식(gzip/snappy/lz4/zstd)별 특성과 실측 벤치마크 결과
- [04-troubleshooting.md](04-troubleshooting.md) — 실습 중 실제로 만난 장애와 해결 기록

각 문서는 실제로 이 저장소의 클러스터를 띄워서 검증한 내용을 기준으로 작성했습니다.
