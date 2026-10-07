# 3노드 KRaft 클러스터 아키텍처

## 왜 3노드인가

처음엔 브로커 1대(콤바인 모드)로 KRaft 동작 원리만 확인하는 구성이었지만, 그것만으로는 **복제(Replication)·리더 선출·장애 조치** 같은 "클러스터다운" 동작을 검증할 수 없었습니다. 그래서 브로커+컨트롤러를 겸하는 콤바인 노드 3대로 쿼럼(Quorum)을 구성했습니다.

- 컨트롤러 쿼럼은 과반수(3대 중 2대)가 살아있어야 리더를 유지/선출할 수 있습니다.
- 이 제약 때문에 노드를 2대만 죽여도 클러스터 전체가 멈추는 걸 실제로 경험했습니다 → [04-troubleshooting.md](04-troubleshooting.md) 참고

## 리스너를 INTERNAL / EXTERNAL로 분리한 이유

```
KAFKA_LISTENERS: INTERNAL://:29092,CONTROLLER://:9093,EXTERNAL://:9092
KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka-1:29092,EXTERNAL://localhost:9092
KAFKA_INTER_BROKER_LISTENER_NAME: INTERNAL
```

브로커 간 복제 트래픽이 `EXTERNAL`(호스트용 `localhost:포트`) 주소를 타면, 그건 컨테이너 밖에서만 유효한 주소라 다른 브로커 컨테이너 안에서는 접속이 안 됩니다. 그래서:
- **INTERNAL**: 도커 네트워크 내부 호스트명(`kafka-1`, `kafka-2`, `kafka-3`)으로 advertise — 브로커 간 복제, `docker exec`로 CLI 실행할 때 사용
- **EXTERNAL**: 호스트에서 접근 가능한 `localhost:9092`/`9094`/`9096`으로 advertise — 맥 터미널이나 호스트에서 직접 붙는 애플리케이션용

`docker exec` 명령어로 카프카 CLI를 실행할 때도 이 구분이 그대로 적용됩니다. 컨테이너 안에서 실행하는 명령은 `--bootstrap-server kafka-1:29092`(INTERNAL)를 써야지, `localhost:9092`를 쓰면 컨테이너 자기 자신만 찾고 다른 브로커의 `localhost:9094` 같은 주소는 컨테이너 안에서 자기 자신을 가리켜서 연결 경고가 뜹니다.

## 이미지: bitnami/kafka → apache/kafka

원래 `bitnami/kafka:3.8.0`으로 실습 가이드가 작성되어 있었는데, 실제로 `docker-compose up`을 돌려보니 pull이 안 됐습니다.

```
Error failed to resolve reference "docker.io/bitnami/kafka:3.8.0": not found
```

확인해보니 Bitnami가 2025년 8월부터 Docker Hub 무료 배포에서 옛날 버전 태그를 전부 내렸습니다(`bitnami/kafka` 태그 목록 조회 시 0건). 그래서 공식 **`apache/kafka`** 이미지로 교체했습니다. 환경변수 네이밍이 달라지는 점에 주의:

| Bitnami | 공식 apache/kafka |
|---|---|
| `KAFKA_CFG_NODE_ID` | `KAFKA_NODE_ID` |
| `KAFKA_CFG_PROCESS_ROLES` | `KAFKA_PROCESS_ROLES` |
| `KAFKA_CFG_LISTENERS` | `KAFKA_LISTENERS` |
| (자동 생성) | `CLUSTER_ID` (직접 지정, 모든 노드 동일 값 필수) |
| `ALLOW_PLAINTEXT_LISTENER=yes` | 불필요 (해당 옵션 자체가 없음) |

CLI 스크립트 경로도 다릅니다: 공식 이미지는 `PATH`에 안 걸려 있어서 `/opt/kafka/bin/kafka-topics.sh`처럼 풀 패스로 호출해야 합니다.

## 관련 파일
- `docker-compose.yml` — 실제 구성
- 루트 `README.md` 1번 섹션 — 그대로 따라 칠 수 있는 실행 커맨드
