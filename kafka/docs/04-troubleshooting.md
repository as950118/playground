# 트러블슈팅 기록

실습 중 실제로 겪은 장애와 원인, 해결 방법을 시간 순으로 정리했습니다. 모두 이 클러스터를 직접 띄워보며 발견한 것들입니다.

## 1. `bitnami/kafka:3.8.0` pull 실패

**증상**
```
Error failed to resolve reference "docker.io/bitnami/kafka:3.8.0": not found
```

**원인**: Bitnami가 2025년 8월부터 Docker Hub 무료 배포에서 옛날 버전 이미지 태그를 전부 내림 (`bitnami/kafka` 태그 목록 조회 시 0건, `bitnamilegacy/kafka`로 이관됨).

**해결**: 공식 `apache/kafka:3.8.0` 이미지로 교체. 환경변수 네이밍이 다르므로 전면 수정 필요 (`KAFKA_CFG_*` → `KAFKA_*`, `CLUSTER_ID` 수동 지정 등). 자세한 내용은 [01-cluster-architecture.md](01-cluster-architecture.md) 참고.

---

## 2. 브로커가 시작하자마자 SIGILL로 죽음

**증상**
```
# A fatal error has been detected by the Java Runtime Environment:
# SIGILL (0x4) at pc=..., pid=12, tid=389
# Problematic frame: j java.lang.System.registerNatives()V+0 java.base
```

**환경**: Apple Silicon Mac + Colima(Virtualization.Framework, aarch64 VM)

**원인**: JDK 21이 가상화 환경에서 SVE(Scalable Vector Extension) 명령어 지원 여부를 오탐지해서 실제로는 지원하지 않는 명령어를 실행하려다 죽는 알려진 문제 (`JDK-8329083` 계열).

**해결**: 컨테이너 환경변수에 `JAVA_TOOL_OPTIONS: "-XX:UseSVE=0"` 추가. 다른 환경(Linux/Intel, Docker Desktop 등)에서는 필요 없는 워크어라운드이므로 이슈 없으면 지워도 됨.

**확인 방법**: `docker run --rm -e JAVA_TOOL_OPTIONS="-XX:UseSVE=0" apache/kafka:3.8.0 java -version`로 정상 기동되는지 먼저 확인.

---

## 3. JBOD 추가 후 볼륨 권한 오류로 브로커 크래시

**증상**
```
Error while writing meta.properties file /var/lib/kafka/data1: /var/lib/kafka/data1/bootstrap.checkpoint.tmp
```

**원인**: `apache/kafka` 이미지는 `appuser`(uid 1000)로 브로커 프로세스를 실행하는데, 새로 생성한 도커 명명 볼륨(`kafka-N-disk1`, `kafka-N-disk2`)은 기본적으로 root 소유(`755`)라 appuser가 쓰기를 못 함. 기존에 쓰던 `/var/lib/kafka/data` 경로는 이미지 안에 미리 존재해서(빌드 시 chown됨) 문제가 없었지만, 새로 추가한 `data1`/`data2` 경로는 이미지에 없는 새 경로라 볼륨 마운트 시 root로 생성됨.

**해결**: 브로커를 띄우기 전에 볼륨 소유권을 미리 맞춰주는 `busybox` 초기화 컨테이너를 추가하고, `depends_on: condition: service_completed_successfully`로 브로커가 그 뒤에 뜨도록 연결.

```yaml
x-volume-init: &volume-init
  image: busybox
  command: sh -c "chown -R 1000:1000 /disk1 /disk2"
```

---

## 4. 방치된 사이 kafka-2/kafka-3가 OOM으로 죽음 (컨트롤러 쿼럼 상실)

**증상**: 몇 시간 뒤 다시 명령을 실행하니 `kafka-2` 관련 명령마다 `UnknownHostException: kafka-2`, 이어서 토픽 관리 명령이 전부 타임아웃.

```
docker ps -a
kafka-1   Up 5 hours
kafka-3   Exited (137) 4 hours ago   # 137 = SIGKILL (OOM-killed)
kafka-2   Exited (137) 3 hours ago
```

**원인**: 이 프로젝트가 돌아가는 Colima VM은 메모리가 **4GiB**로 설정되어 있는데, 같은 VM에 grafana/prometheus/thanos/minio/kind 클러스터 등 다른 프로젝트 컨테이너가 20개 이상 이미 떠 있는 상태(`colima ssh -- free -h` 확인 시 여유 메모리 74~93MiB). 여기에 JVM 브로커 3개(기본 힙 1GB × 3)를 얹으니 감당이 안 돼서 리눅스 OOM killer가 kafka-2/3을 강제 종료함.

컨트롤러 쿼럼은 3대 중 과반수(2대)가 있어야 동작하므로, 브로커 1대만 남은 상태에서는 **kafka-1도 사실상 정상 동작 불가**(리더 선출/커밋 불가) 상태였음.

**해결**: `KAFKA_HEAP_OPTS: "-Xmx512m -Xms512m"`로 브로커당 힙을 512MB까지 낮춰서 3대 합계 메모리 요구량을 줄임. 이후 `docker-compose up -d`로 3대 모두 재기동(볼륨이 살아있어 기존 토픽/데이터 유지된 채로 쿼럼 재구성됨).

**교훈**:
- 공유 개발 환경(Colima 등)에서 여러 프로젝트를 동시에 띄울 땐 `colima ssh -- free -h`로 여유 메모리를 주기적으로 확인할 것.
- 브로커를 함부로 재시작(`docker-compose up -d`)하기 전에, 그게 같은 VM의 **다른 프로젝트 컨테이너까지 OOM으로 끌고 갈 위험**이 있는지 먼저 메모리 여유를 확인하고 진행할 것.
