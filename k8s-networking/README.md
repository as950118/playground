# Kubernetes Networking 동작 분석

## 시험 정보
- **과목**: Network/Security
- **주제**: Kubernetes Networking 동작 분석
- **세부 내용**: kube-proxy의 3가지 모드 동작 방식 상세 분석

## 학습 목표
1. kube-proxy의 3가지 모드 (userspace, iptables, ipvs) 이해
2. 각 모드별 Traffic Flow 분석
3. 모드 간 성능 및 특징 비교
4. 실제 동작 방식 상세 설명

## 학습 구조

### 1. kube-proxy 개요
- kube-proxy의 역할과 기능
- Kubernetes Service와의 관계
- 네트워크 정책 구현 방식

### 2. 각 모드별 상세 분석

#### 2.1 Userspace Mode
- **동작 원리**: 사용자 공간에서 프록시 서비스 제공
- **Traffic Flow**: Pod → kube-proxy → Pod
- **장단점**: 구현 간단, 성능 제한적
- **사용 사례**: 초기 버전, 테스트 환경

#### 2.2 Iptables Mode
- **동작 원리**: Linux iptables를 활용한 네트워크 정책 구현
- **Traffic Flow**: Pod → iptables rules → Pod
- **장단점**: 성능 향상, 복잡한 규칙 관리
- **사용 사례**: 현재 기본 모드, 대부분의 프로덕션 환경

#### 2.3 IPVS Mode
- **동작 원리**: Linux IP Virtual Server를 활용한 로드 밸런싱
- **Traffic Flow**: Pod → ipvs → Pod
- **장단점**: 최고 성능, 대규모 클러스터에 적합
- **사용 사례**: 고성능 요구사항, 대용량 트래픽 처리

### 3. 모드 간 비교 분석

| 구분 | Userspace | Iptables | IPVS |
|------|-----------|----------|------|
| 성능 | 낮음 | 중간 | 높음 |
| 구현 복잡도 | 간단 | 복잡 | 복잡 |
| 메모리 사용량 | 높음 | 중간 | 낮음 |
| 규칙 수 제한 | 없음 | 있음 | 없음 |
| 로드 밸런싱 알고리즘 | 라운드 로빈 | 라운드 로빈 | 다양함 |

### 4. Traffic Flow 상세 분석

#### 4.1 Pod veth에서 Node eth까지의 경로
1. **Pod 내부**: 애플리케이션 → Pod 네트워크 스택
2. **veth pair**: Pod veth → 호스트 veth
3. **호스트 네트워크**: 호스트 veth → kube-proxy 처리
4. **외부 통신**: Node eth → 외부 네트워크

#### 4.2 각 모드별 처리 과정
- **Userspace**: Pod → kube-proxy 프로세스 → 대상 Pod
- **Iptables**: Pod → iptables 규칙 체인 → 대상 Pod
- **IPVS**: Pod → ipvs 테이블 → 대상 Pod

### 5. 실습 및 검증 방법

#### 5.1 환경 구성
```bash
# Minikube로 테스트 환경 구성
minikube start --network-plugin=cni

# kube-proxy 모드 확인
kubectl get configmap -n kube-system kube-proxy -o yaml
```

#### 5.2 모드별 테스트
- 각 모드에서 Service 생성 및 테스트
- 네트워크 정책 적용 테스트
- 성능 벤치마크 측정

### 6. 시험 준비 체크리스트

- [ ] kube-proxy 기본 개념 이해
- [ ] 각 모드의 동작 원리 설명 가능
- [ ] Traffic Flow 상세 설명 가능
- [ ] 모드 간 비교 분석 가능
- [ ] 실제 환경에서 테스트 경험
- [ ] 성능 특성 이해
- [ ] 사용 사례별 모드 선택 기준 이해

## 추가 학습 자료

### 문서
- [Kubernetes 공식 문서 - kube-proxy](https://kubernetes.io/docs/concepts/overview/components/#kube-proxy)
- [Kubernetes 네트워킹 가이드](https://kubernetes.io/docs/concepts/services-networking/)

### 도구
- `kubectl` 명령어를 통한 모드 확인
- `iptables` 명령어를 통한 규칙 확인
- `ipvsadm` 명령어를 통한 IPVS 상태 확인

## 진행 계획

1. **1주차**: 기본 개념 학습 및 환경 구성
2. **2주차**: 각 모드별 상세 분석
3. **3주차**: Traffic Flow 실습 및 검증
4. **4주차**: 모드 간 비교 분석 및 최적화
5. **5주차**: 시험 준비 및 복습

## 실습 환경 구성

### 필수 도구
- Docker Desktop 또는 Minikube
- kubectl CLI
- 네트워크 분석 도구 (tcpdump, wireshark)
- 성능 모니터링 도구

### 테스트 시나리오
1. 기본 Service 생성 및 테스트
2. LoadBalancer Service 테스트
3. NodePort Service 테스트
4. 네트워크 정책 적용 테스트
5. 성능 벤치마크 측정

---

**참고**: 이 문서는 Kubernetes Networking 동작 분석 PT 시험 준비를 위한 가이드입니다. 실제 시험에서는 각 모드의 동작 원리와 Traffic Flow를 상세히 설명할 수 있어야 합니다. 