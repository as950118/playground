# Iptables Mode 상세 분석

## 개요
Iptables Mode는 kube-proxy의 현재 기본 모드로, Linux iptables를 활용하여 네트워크 정책을 구현합니다. 커널 공간에서 동작하여 성능이 향상되었습니다.

## 동작 원리

### 1. 기본 구조
```
Pod A → iptables rules → Pod B
```

### 2. 상세 동작 과정

#### 2.1 Service 생성 시
1. **API Server 감지**: 새로운 Service 생성 감지
2. **iptables 규칙 생성**: NAT 및 포워딩 규칙 생성
3. **체인 구성**: PREROUTING, POSTROUTING, OUTPUT 체인 구성
4. **백엔드 등록**: Service의 Endpoints를 규칙에 등록

#### 2.2 트래픽 처리 과정
```
1. 클라이언트 Pod → Service IP:Port
2. iptables PREROUTING 체인에서 규칙 매칭
3. DNAT 규칙에 의해 백엔드 Pod IP로 변경
4. 라우팅 테이블을 통한 패킷 전달
5. POSTROUTING 체인에서 SNAT 처리
6. 대상 Pod로 트래픽 전달
```

### 3. Traffic Flow 상세 분석

#### 3.1 Pod veth에서 Node eth까지의 경로

```
[Pod A] 
    ↓ (veth pair)
[Host veth] 
    ↓ (iptables PREROUTING)
[Host Network Stack]
    ↓ (iptables NAT)
[Routing Table]
    ↓ (iptables POSTROUTING)
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

**3단계: iptables PREROUTING**
- Service IP:Port에 대한 규칙 매칭
- DNAT 규칙에 의해 백엔드 Pod IP로 변경
- 라운드 로빈 방식으로 백엔드 선택

**4단계: 라우팅 처리**
- 변경된 목적지 IP로 라우팅 테이블 조회
- 대상 Pod가 있는 노드로 패킷 전달

**5단계: iptables POSTROUTING**
- SNAT 규칙에 의해 소스 IP 변경
- 대상 Pod로 최종 전달

### 4. iptables 규칙 구조

#### 4.1 기본 체인 구성
```
nat 테이블:
├── PREROUTING 체인
│   ├── KUBE-SERVICES 체인
│   └── KUBE-POSTROUTING 체인
├── OUTPUT 체인
│   └── KUBE-SERVICES 체인
└── POSTROUTING 체인
    └── KUBE-POSTROUTING 체인

filter 테이블:
├── INPUT 체인
├── FORWARD 체인
└── OUTPUT 체인
```

#### 4.2 Service별 규칙 예시
```bash
# Service 규칙 확인
iptables -t nat -L KUBE-SERVICES -n

# 특정 Service 규칙 확인
iptables -t nat -L KUBE-SVC-XXXXX -n

# 백엔드 규칙 확인
iptables -t nat -L KUBE-SEP-XXXXX -n
```

### 5. 장단점 분석

#### 5.1 장점
- **성능 향상**: 커널 공간에서 동작하여 빠른 처리
- **메모리 효율성**: 규칙 기반 처리로 메모리 사용량 감소
- **안정성**: Linux iptables의 검증된 기술 사용
- **확장성**: 대규모 클러스터에서도 안정적 동작

#### 5.2 단점
- **복잡성**: iptables 규칙 관리의 복잡성
- **디버깅 어려움**: 커널 공간에서 동작하여 추적 어려움
- **규칙 수 제한**: 대량 규칙 시 성능 저하
- **로드 밸런싱 제한**: 라운드 로빈만 지원

### 6. 성능 특성

#### 6.1 처리량
- **최대 처리량**: ~100,000 connections/sec
- **지연시간**: 0.1-1ms (낮은 오버헤드)
- **메모리 사용량**: 규칙당 ~100 bytes

#### 6.2 확장성
- **백엔드 수**: 수천 개까지 가능
- **동시 연결**: 수십만 개까지 가능
- **규칙 수**: 수만 개까지 가능 (성능 저하 가능)

### 7. 사용 사례

#### 7.1 적합한 환경
- **대부분의 프로덕션 환경**: 안정성과 성능의 균형
- **중간 규모 클러스터**: 50-500 노드
- **일반적인 워크로드**: 대부분의 애플리케이션

#### 7.2 부적합한 환경
- **초고성능 요구사항**: IPVS 모드 권장
- **복잡한 로드 밸런싱**: 고급 알고리즘 필요 시
- **대규모 규칙 관리**: 수만 개 규칙 시

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
    mode: "iptables"
    iptables:
      masqueradeAll: true
      masqueradeBit: 14
```

#### 8.2 상태 확인
```bash
# iptables 규칙 확인
iptables -t nat -L -n

# kube-proxy 규칙 확인
iptables -t nat -L KUBE-SERVICES -n

# Service 규칙 확인
kubectl get svc -o wide
```

### 9. 문제 해결

#### 9.1 일반적인 문제
1. **규칙 누락**: Service 생성 시 규칙 생성 실패
2. **성능 저하**: 대량 규칙으로 인한 성능 저하
3. **연결 실패**: NAT 규칙 오류로 인한 연결 실패

#### 9.2 해결 방법
- **규칙 재생성**: kube-proxy 재시작
- **규칙 정리**: 불필요한 규칙 제거
- **성능 모니터링**: 규칙 수 및 처리량 모니터링

### 10. 모니터링 지표

#### 10.1 주요 메트릭
- **규칙 수**: 각 체인별 규칙 수
- **처리량**: 초당 처리 패킷 수
- **지연시간**: 패킷 처리 시간
- **메모리 사용량**: iptables 테이블 메모리 사용량
- **연결 수**: 활성 연결 수

#### 10.2 모니터링 도구
```bash
# iptables 통계 확인
iptables -t nat -L -v

# 규칙 수 확인
iptables -t nat -L | wc -l

# 메모리 사용량 확인
cat /proc/net/ip_tables_matches
```

### 11. 최적화 방안

#### 11.1 성능 최적화
- **규칙 최적화**: 효율적인 규칙 순서 배치
- **백엔드 수 제한**: 적절한 백엔드 수 유지
- **연결 추적**: conntrack 최적화

#### 11.2 운영 최적화
- **정기적인 규칙 정리**: 불필요한 규칙 제거
- **백업 전략**: iptables 규칙 백업
- **모니터링 강화**: 성능 지표 지속적 모니터링

### 12. 고급 기능

#### 12.1 커스텀 규칙
```bash
# 커스텀 규칙 추가
iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 8080

# 규칙 삭제
iptables -t nat -D PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 8080
```

#### 12.2 로드 밸런싱 알고리즘
- **라운드 로빈**: 기본 알고리즘
- **세션 어피니티**: 동일 클라이언트는 동일 백엔드
- **가중치 기반**: 백엔드별 가중치 설정

---

**참고**: Iptables Mode는 현재 Kubernetes의 기본 모드로, 대부분의 프로덕션 환경에서 사용되고 있습니다. 