# Kubernetes Networking 시험 준비 요약

## 시험 정보
- **과목**: Network/Security
- **주제**: Kubernetes Networking 동작 분석
- **세부 내용**: kube-proxy의 3가지 모드 동작 방식 상세 설명

## 핵심 학습 내용

### 1. kube-proxy 개요
- **역할**: Kubernetes Service의 네트워크 프록시
- **기능**: Service IP와 Pod IP 간의 네트워크 통신 중계
- **동작**: 각 노드에서 실행되어 네트워크 정책 구현

### 2. 세 가지 모드 비교

| 구분 | Userspace | Iptables | IPVS |
|------|-----------|----------|------|
| **성능** | 낮음 (~10K conn/sec) | 중간 (~100K conn/sec) | 높음 (~1M conn/sec) |
| **지연시간** | 1-5ms | 0.1-1ms | 0.01-0.1ms |
| **메모리** | 높음 (~1KB/conn) | 중간 (~100B/rule) | 낮음 (~50B/backend) |
| **알고리즘** | 라운드 로빈만 | 라운드 로빈만 | 10가지 알고리즘 |
| **확장성** | 수백 백엔드 | 수천 백엔드 | 수만 백엔드 |
| **복잡도** | 낮음 | 중간 | 높음 |

### 3. 각 모드별 상세 분석

#### 3.1 Userspace Mode
**동작 원리**: 사용자 공간에서 프록시 서비스 제공
```
Pod A → kube-proxy process → Pod B
```

**Traffic Flow**:
1. Pod A에서 Service IP로 요청
2. 호스트의 kube-proxy 포트로 리다이렉트
3. kube-proxy 프로세스가 트래픽 수신
4. 라운드 로빈으로 백엔드 선택
5. 선택된 Pod B로 트래픽 전달

**장점**: 구현 간단, 디버깅 용이
**단점**: 성능 제한, 높은 메모리 사용량

#### 3.2 Iptables Mode
**동작 원리**: Linux iptables를 활용한 네트워크 정책 구현
```
Pod A → iptables rules → Pod B
```

**Traffic Flow**:
1. Pod A에서 Service IP로 요청
2. iptables PREROUTING 체인에서 규칙 매칭
3. DNAT 규칙에 의해 백엔드 Pod IP로 변경
4. 라우팅 테이블을 통한 패킷 전달
5. POSTROUTING 체인에서 SNAT 처리
6. 대상 Pod로 트래픽 전달

**장점**: 성능 향상, 메모리 효율성
**단점**: 복잡한 규칙 관리, 디버깅 어려움

#### 3.3 IPVS Mode
**동작 원리**: Linux IP Virtual Server를 활용한 로드 밸런싱
```
Pod A → ipvs → Pod B
```

**Traffic Flow**:
1. Pod A에서 Service IP로 요청
2. IPVS 가상 서버에서 규칙 매칭
3. 선택된 로드 밸런싱 알고리즘으로 백엔드 결정
4. 백엔드 Pod로 직접 패킷 전달
5. 응답 시 SNAT 처리

**장점**: 최고 성능, 다양한 알고리즘
**단점**: 복잡한 설정, 높은 학습 곡선

### 4. Traffic Flow 상세 분석

#### 4.1 Pod veth에서 Node eth까지의 경로

**공통 경로**:
```
[Pod A] → [veth pair] → [Host Network Stack] → [kube-proxy 처리] → [Host Network Stack] → [veth pair] → [Pod B]
```

**각 모드별 차이점**:
- **Userspace**: 호스트 네트워크 스택 → kube-proxy 프로세스 → 호스트 네트워크 스택
- **Iptables**: 호스트 네트워크 스택 → iptables 규칙 → 라우팅 → iptables 규칙
- **IPVS**: 호스트 네트워크 스택 → IPVS 가상 서버 → 직접 라우팅

#### 4.2 상세 단계별 분석

**1단계: Pod 내부 처리**
- 애플리케이션이 Service IP로 요청
- Pod의 네트워크 스택에서 라우팅 처리

**2단계: veth pair 통과**
- Pod veth → 호스트 veth로 패킷 전달
- 호스트 네트워크 스택으로 진입

**3단계: kube-proxy 처리 (모드별 차이)**
- **Userspace**: iptables 규칙에 의해 kube-proxy 포트로 리다이렉트
- **Iptables**: iptables PREROUTING 체인에서 규칙 매칭
- **IPVS**: IPVS 가상 서버에서 규칙 매칭

**4단계: 백엔드 선택**
- **Userspace**: kube-proxy 프로세스에서 라운드 로빈
- **Iptables**: iptables 규칙에서 라운드 로빈
- **IPVS**: 선택된 알고리즘으로 백엔드 결정

**5단계: 대상 Pod로 전달**
- 선택된 Pod의 IP로 패킷 전송
- 호스트 veth → Pod veth로 전달

### 5. 성능 특성 비교

#### 5.1 처리량 (Throughput)
- **Userspace**: ~10,000 connections/sec
- **Iptables**: ~100,000 connections/sec
- **IPVS**: ~1,000,000 connections/sec

#### 5.2 지연시간 (Latency)
- **Userspace**: 1-5ms (사용자-커널 전환 오버헤드)
- **Iptables**: 0.1-1ms (규칙 매칭 오버헤드)
- **IPVS**: 0.01-0.1ms (직접 패킷 처리)

#### 5.3 확장성
- **Userspace**: 수백 백엔드, 수천 동시 연결
- **Iptables**: 수천 백엔드, 수십만 동시 연결
- **IPVS**: 수만 백엔드, 수백만 동시 연결

### 6. 사용 사례별 권장 모드

#### 6.1 개발/테스트 환경
**권장**: Userspace
**이유**: 설정 간단, 디버깅 용이

#### 6.2 소규모 프로덕션 (10-50 노드)
**권장**: Iptables
**이유**: 안정성과 성능의 균형

#### 6.3 대규모 프로덕션 (500+ 노드)
**권장**: IPVS
**이유**: 최고 성능과 확장성

#### 6.4 고성능 애플리케이션
**권장**: IPVS
**이유**: 최저 지연시간, 최고 처리량

### 7. 설정 및 확인 방법

#### 7.1 모드 설정
```yaml
# kube-proxy ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-proxy
  namespace: kube-system
data:
  config.conf: |
    mode: "userspace"  # 또는 "iptables", "ipvs"
```

#### 7.2 상태 확인
```bash
# Userspace 모드
ps aux | grep kube-proxy

# Iptables 모드
iptables -t nat -L KUBE-SERVICES -n

# IPVS 모드
ipvsadm -L -n
```

### 8. 시험 준비 체크리스트

#### 8.1 이론적 이해
- [ ] kube-proxy의 기본 개념과 역할
- [ ] 세 가지 모드의 동작 원리
- [ ] 각 모드의 장단점
- [ ] Traffic Flow 상세 설명
- [ ] 성능 특성 비교

#### 8.2 실습 능력
- [ ] 각 모드 설정 및 변경
- [ ] 모드별 상태 확인
- [ ] 성능 테스트 수행
- [ ] 문제 진단 및 해결

#### 8.3 시험 준비
- [ ] 각 모드별 상세 설명 준비
- [ ] Traffic Flow 다이어그램 그리기
- [ ] 성능 비교 표 작성
- [ ] 실제 사례 준비

### 9. 핵심 시험 포인트

#### 9.1 반드시 포함될 정보
1. **각 모드간 비교정보**: 성능, 확장성, 기능 비교
2. **각 모드별 Pod veth에서 Node eth까지의 상세 Traffic Flow**: 단계별 패킷 처리 과정
3. **각 모드별 동작 상세정보**: 설정, 확인, 문제 해결 방법

#### 9.2 시험 답변 구조
1. **kube-proxy 개요**: 역할과 기능 설명
2. **세 가지 모드 소개**: 각 모드의 기본 개념
3. **모드별 상세 분석**: 동작 원리와 Traffic Flow
4. **모드 간 비교**: 성능, 확장성, 기능 비교
5. **사용 사례**: 환경별 권장 모드
6. **실습 경험**: 실제 테스트 결과

### 10. 추가 학습 자료

#### 10.1 공식 문서
- [Kubernetes Networking](https://kubernetes.io/docs/concepts/services-networking/)
- [kube-proxy Documentation](https://kubernetes.io/docs/concepts/overview/components/#kube-proxy)

#### 10.2 실습 환경
- Minikube: 로컬 Kubernetes 클러스터
- Docker Desktop: 컨테이너 환경

#### 10.3 도구
- `kubectl`: Kubernetes CLI
- `iptables`: Linux firewall
- `ipvsadm`: IPVS 관리 도구

---

**시험 성공을 위한 팁**:
1. 각 모드의 동작 원리를 정확히 이해
2. Traffic Flow를 단계별로 설명할 수 있도록 준비
3. 실제 실습 경험을 바탕으로 구체적인 예시 제공
4. 성능 비교를 수치로 정확히 기억
5. 사용 사례별 권장 모드를 명확히 설명 