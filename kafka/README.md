# 카프카 KRaft 모드 클러스터 실전 실습 가이드 (Kafka 3.8.0)

본 문서는 주키퍼(Zookeeper) 없이 동작하는 KRaft 모드 **3노드 클러스터**를 구축하고, 복제(Replication)와 장애 조치(Failover)를 포함한 실제 클러스터 동작을 확인하는 가이드입니다. 아래 모든 명령어는 이 저장소의 `docker-compose.yml`로 실제 클러스터를 띄워 직접 검증했습니다.

> 설계 배경(왜 이렇게 구성했는지), 압축 방식 비교, 실습 중 만난 장애와 해결 기록은 [`docs/`](docs/) 폴더에 주제별로 정리되어 있습니다.

## 🛠️ 1. 실습 환경 구축 (Docker Compose)

브로커와 컨트롤러 역할을 함께 수행하는 **콤바인 노드(Combined Mode) 3대**로 쿼럼(Quorum)을 구성합니다. 노드 간 통신(복제)과 호스트 클라이언트 통신을 별도 리스너(`INTERNAL` / `EXTERNAL`)로 분리했고, 브로커 1대당 디스크 2개를 붙인 **JBOD** 구성으로 컨테이너를 재시작해도 데이터가 유지되도록 했습니다.

> **이미지 안내:** 예전에는 `bitnami/kafka` 이미지를 많이 썼지만, Bitnami가 2025년 8월부터 Docker Hub 무료 배포에서 예전 버전 태그들을 내리면서 `bitnami/kafka:3.8.0` 등은 더 이상 받을 수 없습니다. 이 가이드는 공식 **`apache/kafka`** 이미지를 사용합니다.

### 1) `docker-compose.yml` 파일 작성
실습할 디렉토리에 아래 내용을 `docker-compose.yml`로 저장합니다. (이 저장소 루트에 이미 포함되어 있습니다.)

```yaml
x-kafka-common: &kafka-common
  image: apache/kafka:3.8.0
  networks:
    - kafka-net
  environment: &kafka-common-env
    # Apple Silicon + Colima(QEMU) 환경에서 JDK 21이 SVE 명령어를 오탐지해 SIGILL로 죽는
    # 문제를 우회합니다. (https://bugs.openjdk.org/browse/JDK-8329083 계열 이슈)
    # 다른 환경(Linux/Intel, Docker Desktop 등)에서는 필요 없으면 이 줄을 지워도 됩니다.
    JAVA_TOOL_OPTIONS: "-XX:UseSVE=0"
    # 개발용 VM의 메모리가 넉넉하지 않다면 브로커 기본 힙(1GB)이 부담될 수 있습니다.
    # 실습 목적으로는 512m로도 충분합니다. (자세한 내용: docs/04-troubleshooting.md)
    KAFKA_HEAP_OPTS: "-Xmx512m -Xms512m"
    KAFKA_PROCESS_ROLES: broker,controller
    # 클러스터를 구성하는 모든 노드는 동일한 CLUSTER_ID를 공유해야 합니다.
    CLUSTER_ID: "MkU3OEVBNTcwNTJENDM2Qk"
    KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
    # INTERNAL: 브로커 간 통신(복제) / EXTERNAL: 호스트에서 접속하는 클라이언트용
    KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: INTERNAL:PLAINTEXT,EXTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
    KAFKA_INTER_BROKER_LISTENER_NAME: INTERNAL
    KAFKA_CONTROLLER_QUORUM_VOTERS: "1@kafka-1:9093,2@kafka-2:9093,3@kafka-3:9093"
    # JBOD: 브로커 1대당 디스크(로그 디렉터리) 2개를 등록합니다. 새 파티션은 이 목록에
    # 라운드로빈으로 배정되고, 파티션 단위로 어느 디스크에 있는지는
    # `kafka-log-dirs.sh --describe`로 확인할 수 있습니다.
    KAFKA_LOG_DIRS: /var/lib/kafka/data1,/var/lib/kafka/data2

x-volume-init: &volume-init
  image: busybox
  # apache/kafka 이미지는 appuser(uid 1000)로 브로커를 실행하는데, 새로 생성된 도커
  # 볼륨은 기본적으로 root 소유라 브로커가 로그 디렉터리에 쓰지 못하고 죽습니다.
  # 브로커를 띄우기 전에 각 디스크 볼륨의 소유권을 appuser로 미리 맞춰줍니다.
  command: sh -c "chown -R 1000:1000 /disk1 /disk2"

services:
  kafka-1-init:
    <<: *volume-init
    volumes:
      - kafka-1-disk1:/disk1
      - kafka-1-disk2:/disk2

  kafka-2-init:
    <<: *volume-init
    volumes:
      - kafka-2-disk1:/disk1
      - kafka-2-disk2:/disk2

  kafka-3-init:
    <<: *volume-init
    volumes:
      - kafka-3-disk1:/disk1
      - kafka-3-disk2:/disk2

  kafka-1:
    <<: *kafka-common
    container_name: kafka-1
    depends_on:
      kafka-1-init:
        condition: service_completed_successfully
    ports:
      - "9092:9092"
    environment:
      <<: *kafka-common-env
      KAFKA_NODE_ID: 1
      KAFKA_LISTENERS: INTERNAL://:29092,CONTROLLER://:9093,EXTERNAL://:9092
      KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka-1:29092,EXTERNAL://localhost:9092
    volumes:
      - kafka-1-disk1:/var/lib/kafka/data1
      - kafka-1-disk2:/var/lib/kafka/data2

  kafka-2:
    <<: *kafka-common
    container_name: kafka-2
    depends_on:
      kafka-2-init:
        condition: service_completed_successfully
    ports:
      - "9094:9092"
    environment:
      <<: *kafka-common-env
      KAFKA_NODE_ID: 2
      KAFKA_LISTENERS: INTERNAL://:29092,CONTROLLER://:9093,EXTERNAL://:9092
      KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka-2:29092,EXTERNAL://localhost:9094
    volumes:
      - kafka-2-disk1:/var/lib/kafka/data1
      - kafka-2-disk2:/var/lib/kafka/data2

  kafka-3:
    <<: *kafka-common
    container_name: kafka-3
    depends_on:
      kafka-3-init:
        condition: service_completed_successfully
    ports:
      - "9096:9092"
    environment:
      <<: *kafka-common-env
      KAFKA_NODE_ID: 3
      KAFKA_LISTENERS: INTERNAL://:29092,CONTROLLER://:9093,EXTERNAL://:9092
      KAFKA_ADVERTISED_LISTENERS: INTERNAL://kafka-3:29092,EXTERNAL://localhost:9096
    volumes:
      - kafka-3-disk1:/var/lib/kafka/data1
      - kafka-3-disk2:/var/lib/kafka/data2

networks:
  kafka-net:
    driver: bridge

volumes:
  kafka-1-disk1:
  kafka-1-disk2:
  kafka-2-disk1:
  kafka-2-disk2:
  kafka-3-disk1:
  kafka-3-disk2:
```

> **왜 리스너를 분리했나요?** 브로커 간 복제 트래픽이 `EXTERNAL` 리스너(호스트용 주소)를 타면 컨테이너 밖 주소로는 서로 접속할 수 없어 복제가 깨집니다. `INTERNAL`은 도커 네트워크 내부 호스트명(`kafka-1` 등)으로, `EXTERNAL`은 호스트에서 접근 가능한 `localhost:포트`로 advertise하도록 분리했습니다.

> **왜 디스크(볼륨)를 2개씩 붙였나요?** 브로커 1대에 여러 디스크를 등록하는 JBOD(Just a Bunch Of Disks) 구성입니다. 브로커를 늘리지 않고도 저장 용량/처리량을 늘릴 수 있지만, 그 브로커가 있는 VM 자체가 죽으면 디스크 개수와 상관없이 그 브로커는 통째로 사라진다는 한계가 있습니다(장애 격리는 여전히 브로커 단위). 자세한 내용은 아래 5번 섹션 참고. 볼륨을 쓰는 이유 자체는 `docker-compose down` 후에도 데이터/클러스터 상태를 유지하기 위함입니다.

### 2) 카프카 클러스터 실행
```bash
docker-compose up -d
```
> **확인 방법:** `docker logs -f kafka-1`(혹은 -2, -3)를 실행하여 세 노드 모두 에러 없이 정상적으로 시작되었는지 확인합니다.

---

## 💻 2. 데이터 송수신 & 복제 실습

**'kakao'** 토픽을 파티션 3개, **복제 계수(replication-factor) 3**으로 생성하여 클러스터 전체에 데이터가 복제되는지 확인합니다.

> **참고:** 아래 `docker exec` 명령어는 컨테이너 안에서 CLI를 실행하므로, `--bootstrap-server`는 **도커 네트워크 내부 주소(`kafka-N:29092`, `INTERNAL` 리스너)** 를 사용합니다. 컨테이너 내부에서 `localhost:9092`를 쓰면 자기 자신은 찾지만, 다른 브로커의 `EXTERNAL` 주소(`localhost:9094`, `9096`)는 컨테이너 안에서는 자기 자신을 가리켜 연결 경고가 발생합니다. 반대로 호스트(맥/터미널)에 카프카 클라이언트가 설치되어 있어 직접 접속한다면 `localhost:9092`/`9094`/`9096`을 그대로 쓰면 됩니다.

### 1) 토픽 생성
```bash
docker exec -it kafka-1 /opt/kafka/bin/kafka-topics.sh --create \
  --topic kakao \
  --partitions 3 \
  --replication-factor 3 \
  --bootstrap-server kafka-1:29092
```

### 2) 토픽 상세 정보로 복제 상태 확인
파티션별 리더(Leader)와 복제본(Replicas), ISR(In-Sync Replicas)이 3개 브로커에 고르게 분산되어 있는지 확인합니다.
```bash
docker exec -it kafka-1 /opt/kafka/bin/kafka-topics.sh --describe \
  --topic kakao \
  --bootstrap-server kafka-1:29092
```
**[실제 출력 예시]**
```text
Topic: kakao	TopicId: QrCywPFeTuu80sHW-mISxQ	PartitionCount: 3	ReplicationFactor: 3	Configs:
	Topic: kakao	Partition: 0	Leader: 2	Replicas: 2,3,1	Isr: 2,3,1
	Topic: kakao	Partition: 1	Leader: 3	Replicas: 3,1,2	Isr: 3,1,2
	Topic: kakao	Partition: 2	Leader: 1	Replicas: 1,2,3	Isr: 1,2,3
```
파티션마다 리더가 서로 다른 브로커(2, 3, 1)로 분산되고, `Replicas`/`Isr`에 3개 브로커가 모두 들어있으면 복제가 정상 동작하는 것입니다.

### 3) 프로듀서(Producer)로 메시지 전송
아래 명령어로 프로듀서 쉘에 접속합니다. (어느 브로커에 접속해도 클러스터 전체에 기록됩니다)
```bash
docker exec -it kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --topic kakao \
  --bootstrap-server kafka-1:29092
```
`>` 프롬프트가 나타나면 아래 JSON 데이터를 한 줄씩 입력합니다.
```json
{"event": "login", "user": "user_01", "timestamp": "2024-11-06T10:00:00Z"}
{"event": "click_ad", "user": "user_01", "ad_id": "kafka_kraft_promo"}
{"event": "purchase", "user": "user_02", "item": "americano", "price": 4500}
```

### 4) 컨슈머(Consumer)로 메시지 수신
새로운 터미널을 열고, 이번엔 **다른 브로커(kafka-2)** 에 접속해 데이터를 확인합니다. 클러스터 전체에서 동일한 데이터가 조회되는지 확인하는 것이 포인트입니다.
```bash
docker exec -it kafka-2 /opt/kafka/bin/kafka-console-consumer.sh \
  --topic kakao \
  --from-beginning \
  --bootstrap-server kafka-2:29092
```
**[예상 출력]**
프로듀서에서 입력한 이벤트 로그 3줄이 즉각적으로 출력됩니다.

---

## 🔥 3. 장애 조치(Failover) 실습

리더 역할을 맡고 있는 브로커 하나를 강제로 중단시켜, 남은 브로커 중에서 새로운 리더가 자동으로 선출되는지 확인합니다.

```bash
# 2) 에서 확인한 리더 브로커를 중단 (예: kafka-2가 리더인 경우)
docker stop kafka-2

# 잠시 후 다시 토픽 상세 정보 조회 → 파티션 리더가 살아있는 노드로 재선출되었는지 확인
docker exec -it kafka-1 /opt/kafka/bin/kafka-topics.sh --describe \
  --topic kakao \
  --bootstrap-server kafka-1:29092
```
`kafka-2`가 리더였던 파티션의 `Leader`가 살아있는 다른 브로커(1 또는 3)로 바뀌고, `Isr`에서 `kafka-2`(브로커 2)가 잠시 빠지는 것을 확인할 수 있습니다.

```bash
# 복구 - 다시 시작하면 잠시 후 Isr에 재합류합니다
docker start kafka-2
```

---

## 💽 4. JBOD 실습: 파티션이 어느 디스크에 저장되는지 확인

`kafka-log-dirs.sh --describe`로 각 브로커가 파티션(정확히는 그 브로커가 들고 있는 복제본)을 `data1`/`data2` 중 어디에 저장했는지 확인합니다.

```bash
docker exec -it kafka-1 /opt/kafka/bin/kafka-log-dirs.sh --describe \
  --bootstrap-server kafka-1:29092 \
  --topic-list kakao
```
**[실제 출력 예시]** (가독성을 위해 정리함)
```text
broker 1: data1 → [kakao-0, kakao-2], data2 → [kakao-1]
broker 2: data1 → [kakao-1, kakao-2], data2 → [kakao-0]
broker 3: data1 → [kakao-1, kakao-2], data2 → [kakao-0]
```
브로커마다 3개 파티션 복제본이 `data1`/`data2` 두 디스크에 라운드로빈으로 나뉘어 저장된 것을 볼 수 있습니다. 컨테이너 안에서 직접 디렉터리를 봐도 동일하게 확인됩니다.
```bash
docker exec kafka-1 ls /var/lib/kafka/data1   # kakao-0, kakao-2, __cluster_metadata-0 ...
docker exec kafka-1 ls /var/lib/kafka/data2   # kakao-1 ...
```

> **한계:** 위에서 언급했듯 JBOD는 "한 브로커의 저장 공간"을 늘리는 것이지, "장애 격리 단위"를 늘리는 게 아닙니다. `data1`, `data2` 중 디스크 하나만 죽어도 (KRaft에서는 브로커 전체가 아니라 그 디스크의 파티션만) 오프라인이 되지만, 컨테이너/VM 자체가 죽으면 두 디스크 다 같이 못 씁니다. 진짜 장애 격리를 보려면 3번 섹션의 브로커 단위 장애 조치 실습을 참고하세요.

---

## 🔍 5. 심화 실습: KRaft 메타데이터 확인

주키퍼 대신 카프카 내부에 생성된 `__cluster_metadata` 토픽의 내용을 덤프(Dump) 명령어로 직접 확인합니다. `__cluster_metadata` 파티션은 컨트롤러 쿼럼 전용이라 `KAFKA_LOG_DIRS`에 지정한 첫 번째 디스크(`data1`)에 생성됩니다. 아래는 `kafka-1` 노드 기준이며, 다른 노드에서 실행해도 (컨트롤러 쿼럼을 통해 복제된) 동일한 메타데이터를 볼 수 있습니다.

```bash
docker exec -it kafka-1 /opt/kafka/bin/kafka-dump-log.sh \
  --cluster-metadata-decoder \
  --files /var/lib/kafka/data1/__cluster_metadata-0/00000000000000000000.log
```

**[실제 출력 예시]**
앞서 생성했던 `kakao` 토픽의 메타데이터와 파티션별 리더/복제본 정보가 JSON 형태로 출력됩니다.
```text
| payload: {"type":"TOPIC_RECORD","version":0,"data":{"name":"kakao","topicId":"QrCywPFeTuu80sHW-mISxQ"}}
| payload: {"type":"PARTITION_RECORD","version":1,"data":{"partitionId":0,"topicId":"QrCywPFeTuu80sHW-mISxQ","replicas":[2,3,1],"isr":[2,3,1],"leader":2,"leaderEpoch":0,"partitionEpoch":0, ...}}
| payload: {"type":"PARTITION_RECORD","version":1,"data":{"partitionId":1,"topicId":"QrCywPFeTuu80sHW-mISxQ","replicas":[3,1,2],"isr":[3,1,2],"leader":3,"leaderEpoch":0,"partitionEpoch":0, ...}}
| payload: {"type":"PARTITION_RECORD","version":1,"data":{"partitionId":2,"topicId":"QrCywPFeTuu80sHW-mISxQ","replicas":[1,2,3],"isr":[1,2,3],"leader":1,"leaderEpoch":0,"partitionEpoch":0, ...}}
```

---
*참고 영상: [카프카, KRaft를 만나다 / if(kakaoAI)2024](https://www.youtube.com/watch?v=VIGkd2U_8Ro)*
