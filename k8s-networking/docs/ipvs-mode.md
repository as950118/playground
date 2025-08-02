# IPVS Mode 상세 분석

## 개요
IPVS (IP Virtual Server) Mode는 kube-proxy의 최고 성능 모드로, Linux IPVS를 활용하여 고성능 로드 밸런싱을 제공합니다. 대규모 클러스터와 고성능 요구사항에 최적화되어 있습니다.

## 동작 원리

### 1. 기본 구조
```
Pod A → ipvs → Pod B
```

### 2. 상세 동작 과정

#### 2.1 Service 생성 시
1. **API Server 감지**: 새로운 Service 생성 감지
2. **IPVS 테이블 생성**: 가상 서버 및 백엔드 서버 등록
3. **네트워크 인터페이스 설정**: 가상 IP 바인딩
4. **로드 밸런싱 알고리즘 설정**: 선택된 알고리즘 적용

#### 2.2 트래픽 처리 과정
```
1. 클라이언트 Pod → Service IP:Port
2. IPVS 가상 서버에서 규칙 매칭
3. 선택된 로드 밸런싱 알고리즘으로 백엔드 결정
4. 백엔드 Pod로 직접 패킷 전달
5. 응답 시 SNAT 처리
```

### 3. Traffic Flow 상세 분석

#### 3.1 Pod veth에서 Node eth까지의 경로

```
[Pod A] 
    ↓ (veth pair)
[Host veth] 
    ↓ (ipvs virtual server)
[Host Network Stack]
    ↓ (direct routing)
[Host veth]
    ↓ (veth pair)
[Pod B]
```

#### 3.2 상세 단계별 분석

**1단계: Pod 내부 처리**
- 애플리케이션이 Service IP로 요청
- Pod의 네트워크 스택에서 라우팅 처리

**2단계: veth pair 통과**
- Pod veth → 호스트 veth로 패킷 전달
- 호스트 네트워크 스택으로 진입

**3단계: IPVS 가상 서버**
- Service IP:Port에 대한 가상 서버 매칭
- 설정된 로드 밸런싱 알고리즘으로 백엔드 선택
- 백엔드 Pod IP로 패킷 헤더 변경

**4단계: 직접 라우팅**
- 변경된 목적지 IP로 직접 라우팅
- 대상 Pod로 패킷 전달

**5단계: 응답 처리**
- 응답 시 SNAT 규칙에 의해 소스 IP 변경
- 클라이언트 Pod로 응답 전달

### 4. IPVS 구성 요소

#### 4.1 가상 서버 (Virtual Server)
```bash
# 가상 서버 확인
ipvsadm -L -n

# 특정 Service 가상 서버 확인
ipvsadm -L -n | grep <service-ip>
```

#### 4.2 백엔드 서버 (Real Server)
```bash
# 백엔드 서버 확인
ipvsadm -L -n --real

# 특정 Service 백엔드 확인
ipvsadm -L -n | grep -A 10 <service-ip>
```

#### 4.3 로드 밸런싱 알고리즘
- **rr (Round Robin)**: 기본 라운드 로빈
- **wrr (Weighted Round Robin)**: 가중치 기반 라운드 로빈
- **lc (Least Connection)**: 최소 연결 수
- **wlc (Weighted Least Connection)**: 가중치 기반 최소 연결
- **lblc (Locality-Based Least Connection)**: 지역성 기반 최소 연결
- **lblcr (Locality-Based Least Connection with Replication)**: 복제 기반 지역성
- **dh (Destination Hashing)**: 목적지 해싱
- **sh (Source Hashing)**: 소스 해싱
- **sed (Shortest Expected Delay)**: 최단 예상 지연
- **nq (Never Queue)**: 대기열 없음

### 5. 장단점 분석

#### 5.1 장점
- **최고 성능**: 커널 공간에서 직접 처리
- **다양한 알고리즘**: 10가지 로드 밸런싱 알고리즘 지원
- **확장성**: 대규모 클러스터에서도 안정적
- **메모리 효율성**: 매우 낮은 메모리 사용량
- **연결 추적**: 효율적인 연결 상태 관리

#### 5.2 단점
- **복잡성**: IPVS 설정의 복잡성
- **디버깅 어려움**: 커널 모듈 기반으로 추적 어려움
- **호환성**: 일부 네트워크 플러그인과 호환성 문제
- **학습 곡선**: IPVS 개념 이해 필요

### 6. 성능 특성

#### 6.1 처리량
- **최대 처리량**: ~1,000,000 connections/sec
- **지연시간**: 0.01-0.1ms (매우 낮은 오버헤드)
- **메모리 사용량**: 백엔드당 ~50 bytes

#### 6.2 확장성
- **백엔드 수**: 수만 개까지 가능
- **동시 연결**: 수백만 개까지 가능
- **가상 서버 수**: 수천 개까지 가능

### 7. 사용 사례

#### 7.1 적합한 환경
- **대규모 프로덕션 환경**: 500+ 노드 클러스터
- **고성능 애플리케이션**: 낮은 지연시간 요구
- **대용량 트래픽**: 초당 수만 연결 처리
- **복잡한 로드 밸런싱**: 고급 알고리즘 필요

#### 7.2 부적합한 환경
- **소규모 클러스터**: 오버헤드가 불필요
- **간단한 워크로드**: 기본 로드 밸런싱만 필요
- **리소스 제한 환경**: 메모리/CPU 제한

### 8. 설정 및 확인 방법

#### 8.1 모드 설정
```yaml
# kube-proxy ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-proxy
  namespace: kube-system
data:
  config.conf: |
    mode: "ipvs"
    ipvs:
      scheduler: "rr"
      excludeCIDRs: null
      strictARP: false
      tcpTimeout: 0
      tcpFinTimeout: 0
      udpTimeout: 0
```

#### 8.2 상태 확인
```bash
# IPVS 상태 확인
ipvsadm -L -n

# 가상 서버 통계 확인
ipvsadm -L -n --stats

# 백엔드 서버 상태 확인
ipvsadm -L -n --real

# 연결 상태 확인
ipvsadm -L -n --connection
```

### 9. 문제 해결

#### 9.1 일반적인 문제
1. **IPVS 모듈 누락**: ip_vs 모듈 로드 필요
2. **네트워크 인터페이스 문제**: 가상 IP 바인딩 실패
3. **백엔드 등록 실패**: Pod IP 등록 실패
4. **성능 저하**: 알고리즘 선택 문제

#### 9.2 해결 방법
```bash
# IPVS 모듈 로드
modprobe ip_vs
modprobe ip_vs_rr
modprobe ip_vs_wrr
modprobe ip_vs_lc

# IPVS 상태 확인
lsmod | grep ip_vs

# 네트워크 인터페이스 확인
ip addr show

# kube-proxy 재시작
kubectl delete pod -n kube-system -l k8s-app=kube-proxy
```

### 10. 모니터링 지표

#### 10.1 주요 메트릭
- **가상 서버 수**: 등록된 가상 서버 수
- **백엔드 서버 수**: 각 가상 서버별 백엔드 수
- **연결 수**: 활성 연결 수
- **처리량**: 초당 처리 패킷 수
- **지연시간**: 패킷 처리 시간
- **메모리 사용량**: IPVS 테이블 메모리 사용량

#### 10.2 모니터링 도구
```bash
# IPVS 통계 확인
ipvsadm -L -n --stats

# 연결 상태 확인
ipvsadm -L -n --connection

# 메모리 사용량 확인
cat /proc/net/ip_vs/stats
```

### 11. 최적화 방안

#### 11.1 성능 최적화
- **알고리즘 선택**: 워크로드에 맞는 알고리즘 선택
- **백엔드 수 최적화**: 적절한 백엔드 수 유지
- **연결 추적 최적화**: conntrack 설정 최적화
- **네트워크 인터페이스 최적화**: 가상 IP 바인딩 최적화

#### 11.2 운영 최적화
- **정기적인 상태 확인**: IPVS 상태 모니터링
- **백업 전략**: IPVS 설정 백업
- **업그레이드 계획**: IPVS 모듈 업그레이드 계획

### 12. 고급 기능

#### 12.1 커스텀 스케줄러
```bash
# 커스텀 가상 서버 생성
ipvsadm -A -t <service-ip>:<port> -s <scheduler>

# 백엔드 서버 추가
ipvsadm -a -t <service-ip>:<port> -r <backend-ip>:<port> -m

# 가상 서버 삭제
ipvsadm -D -t <service-ip>:<port>
```

#### 12.2 고급 알고리즘 활용
- **지역성 기반**: lblc, lblcr 알고리즘
- **해싱 기반**: dh, sh 알고리즘
- **지연시간 기반**: sed, nq 알고리즘

### 13. IPVS vs Iptables 비교

| 구분 | IPVS | Iptables |
|------|------|----------|
| 성능 | 최고 (1M+ conn/sec) | 중간 (100K+ conn/sec) |
| 메모리 사용량 | 매우 낮음 | 낮음 |
| 알고리즘 | 10가지 | 라운드 로빈만 |
| 확장성 | 수만 백엔드 | 수천 백엔드 |
| 복잡성 | 높음 | 중간 |
| 디버깅 | 어려움 | 어려움 |

---

**참고**: IPVS Mode는 고성능 요구사항이 있는 대규모 프로덕션 환경에서 권장되는 모드입니다. 