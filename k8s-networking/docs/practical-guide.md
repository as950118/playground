# Kubernetes Networking 실습 가이드

## 개요
kube-proxy의 세 가지 모드를 실제 환경에서 테스트하고 분석하는 실습 가이드를 제공합니다.

## 1. 실습 환경 구성

### 1.1 필수 도구 설치
```bash
# Docker Desktop 또는 Minikube 설치
# macOS
brew install --cask docker

# 또는 Minikube
brew install minikube

# kubectl 설치
brew install kubectl

# 네트워크 분석 도구
brew install tcpdump
brew install wireshark

# 성능 테스트 도구
brew install apache2-utils  # ab (Apache Bench)
```

### 1.2 클러스터 구성
```bash
# Minikube 시작 (기본 iptables 모드)
minikube start --network-plugin=cni

# 또는 IPVS 모드로 시작
minikube start --network-plugin=cni --extra-config=kube-proxy.mode=ipvs

# 클러스터 상태 확인
kubectl cluster-info
kubectl get nodes
```

## 2. kube-proxy 모드 확인 및 변경

### 2.1 현재 모드 확인
```bash
# kube-proxy ConfigMap 확인
kubectl get configmap -n kube-system kube-proxy -o yaml

# kube-proxy Pod 로그 확인
kubectl logs -n kube-system -l k8s-app=kube-proxy

# 모드별 확인 명령어
# Userspace 모드
ps aux | grep kube-proxy

# Iptables 모드
iptables -t nat -L KUBE-SERVICES -n

# IPVS 모드
ipvsadm -L -n
```

### 2.2 모드 변경 실습
```bash
# 1. 현재 설정 백업
kubectl get configmap -n kube-system kube-proxy -o yaml > kube-proxy-backup.yaml

# 2. ConfigMap 편집
kubectl edit configmap -n kube-system kube-proxy

# 3. kube-proxy 재시작
kubectl delete pod -n kube-system -l k8s-app=kube-proxy

# 4. 변경 확인
kubectl logs -n kube-system -l k8s-app=kube-proxy
```

## 3. 각 모드별 실습

### 3.1 Userspace Mode 실습

#### 3.1.1 모드 설정
```yaml
# kube-proxy ConfigMap 설정
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-proxy
  namespace: kube-system
data:
  config.conf: |
    mode: "userspace"
```

#### 3.1.2 테스트 애플리케이션 배포
```bash
# 테스트 Pod 생성
kubectl run nginx --image=nginx --port=80 --replicas=3

# Service 생성
kubectl expose deployment nginx --port=80 --target-port=80

# 테스트 클라이언트 생성
kubectl run test-client --image=busybox --command -- sleep 3600
```

#### 3.1.3 Traffic Flow 분석
```bash
# Pod 간 통신 테스트
kubectl exec test-client -- wget -O- http://nginx/

# 네트워크 패킷 캡처
kubectl exec test-client -- tcpdump -i any -w /tmp/capture.pcap &
kubectl exec test-client -- wget -O- http://nginx/
kubectl cp test-client:/tmp/capture.pcap ./userspace-capture.pcap

# 프로세스 확인
kubectl exec test-client -- ps aux | grep kube-proxy
```

### 3.2 Iptables Mode 실습

#### 3.2.1 모드 설정
```yaml
# kube-proxy ConfigMap 설정
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

#### 3.2.2 iptables 규칙 분석
```bash
# Service 규칙 확인
iptables -t nat -L KUBE-SERVICES -n

# 특정 Service 규칙 확인
kubectl get svc nginx -o wide
iptables -t nat -L KUBE-SVC-XXXXX -n

# 백엔드 규칙 확인
iptables -t nat -L KUBE-SEP-XXXXX -n

# 규칙 통계 확인
iptables -t nat -L -v
```

#### 3.2.3 성능 테스트
```bash
# 처리량 테스트
kubectl exec test-client -- ab -n 10000 -c 100 http://nginx/

# 지연시간 테스트
kubectl exec test-client -- ping nginx

# 연결 수 테스트
for i in {1..100}; do
  kubectl exec test-client -- wget -O- http://nginx/ &
done
```

### 3.3 IPVS Mode 실습

#### 3.3.1 사전 준비
```bash
# IPVS 모듈 로드
sudo modprobe ip_vs
sudo modprobe ip_vs_rr
sudo modprobe ip_vs_wrr
sudo modprobe ip_vs_lc

# 모듈 확인
lsmod | grep ip_vs
```

#### 3.3.2 모드 설정
```yaml
# kube-proxy ConfigMap 설정
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
```

#### 3.3.3 IPVS 상태 분석
```bash
# 가상 서버 확인
ipvsadm -L -n

# 백엔드 서버 확인
ipvsadm -L -n --real

# 통계 확인
ipvsadm -L -n --stats

# 연결 상태 확인
ipvsadm -L -n --connection
```

#### 3.3.4 고급 기능 테스트
```bash
# 다양한 스케줄러 테스트
# Round Robin
kubectl patch svc nginx -p '{"spec":{"sessionAffinity":"None"}}'

# Least Connection
# (IPVS에서 지원하는 경우)

# 가중치 기반 테스트
# (고급 설정 필요)
```

## 4. 성능 벤치마크

### 4.1 테스트 시나리오
```bash
# 1. 기본 연결 테스트
kubectl exec test-client -- wget -O- http://nginx/

# 2. 동시 연결 테스트
for i in {1..50}; do
  kubectl exec test-client -- wget -O- http://nginx/ &
done

# 3. 처리량 테스트
kubectl exec test-client -- ab -n 10000 -c 100 http://nginx/

# 4. 지연시간 테스트
kubectl exec test-client -- ping nginx
```

### 4.2 성능 측정 스크립트
```bash
#!/bin/bash
# performance-test.sh

SERVICE_NAME="nginx"
TEST_DURATION=60
CONCURRENT_USERS=100
TOTAL_REQUESTS=10000

echo "=== Kubernetes Networking Performance Test ==="
echo "Service: $SERVICE_NAME"
echo "Duration: ${TEST_DURATION}s"
echo "Concurrent Users: $CONCURRENT_USERS"
echo "Total Requests: $TOTAL_REQUESTS"

# 처리량 테스트
echo "=== Throughput Test ==="
kubectl exec test-client -- ab -n $TOTAL_REQUESTS -c $CONCURRENT_USERS http://$SERVICE_NAME/

# 지연시간 테스트
echo "=== Latency Test ==="
kubectl exec test-client -- ping -c 10 $SERVICE_NAME

# 연결 수 테스트
echo "=== Connection Test ==="
kubectl exec test-client -- netstat -an | grep ESTABLISHED | wc -l
```

### 4.3 모니터링 및 분석
```bash
# 리소스 사용량 모니터링
kubectl top pods -n default

# 네트워크 통계 확인
kubectl exec test-client -- netstat -i

# 프로세스 모니터링
kubectl exec test-client -- ps aux | grep -E "(nginx|kube-proxy)"

# 로그 분석
kubectl logs -l app=nginx
kubectl logs -n kube-system -l k8s-app=kube-proxy
```

## 5. 문제 해결 실습

### 5.1 일반적인 문제 시나리오

#### 5.1.1 Service 연결 실패
```bash
# 문제 진단
kubectl get svc nginx
kubectl get endpoints nginx
kubectl describe svc nginx

# 네트워크 연결 테스트
kubectl exec test-client -- nslookup nginx
kubectl exec test-client -- telnet nginx 80
```

#### 5.1.2 kube-proxy 문제
```bash
# kube-proxy 상태 확인
kubectl get pods -n kube-system -l k8s-app=kube-proxy
kubectl logs -n kube-system -l k8s-app=kube-proxy

# 설정 확인
kubectl get configmap -n kube-system kube-proxy -o yaml
```

#### 5.1.3 네트워크 정책 문제
```bash
# iptables 규칙 확인
iptables -t nat -L -n

# IPVS 상태 확인
ipvsadm -L -n

# 네트워크 인터페이스 확인
ip addr show
```

### 5.2 디버깅 도구 활용
```bash
# tcpdump를 통한 패킷 분석
kubectl exec test-client -- tcpdump -i any -w /tmp/debug.pcap &
kubectl exec test-client -- wget -O- http://nginx/
kubectl cp test-client:/tmp/debug.pcap ./debug-capture.pcap

# strace를 통한 시스템 콜 추적
kubectl exec test-client -- strace -f wget -O- http://nginx/
```

## 6. 시험 준비 체크리스트

### 6.1 이론적 이해
- [ ] kube-proxy의 기본 개념과 역할
- [ ] 세 가지 모드의 동작 원리
- [ ] 각 모드의 장단점
- [ ] Traffic Flow 상세 설명
- [ ] 성능 특성 비교

### 6.2 실습 능력
- [ ] 각 모드 설정 및 변경
- [ ] 모드별 상태 확인
- [ ] 성능 테스트 수행
- [ ] 문제 진단 및 해결
- [ ] 모니터링 및 분석

### 6.3 시험 준비
- [ ] 각 모드별 상세 설명 준비
- [ ] Traffic Flow 다이어그램 그리기
- [ ] 성능 비교 표 작성
- [ ] 실제 사례 준비
- [ ] 질문 대응 연습

## 7. 추가 학습 자료

### 7.1 공식 문서
- [Kubernetes Networking](https://kubernetes.io/docs/concepts/services-networking/)
- [kube-proxy Documentation](https://kubernetes.io/docs/concepts/overview/components/#kube-proxy)
- [IPVS Documentation](http://www.linuxvirtualserver.org/)

### 7.2 도구 및 유틸리티
- `kubectl`: Kubernetes CLI
- `iptables`: Linux firewall
- `ipvsadm`: IPVS 관리 도구
- `tcpdump`: 패킷 캡처
- `wireshark`: 패킷 분석
- `ab`: Apache Bench (성능 테스트)

### 7.3 실습 환경
- Minikube: 로컬 Kubernetes 클러스터
- Docker Desktop: 컨테이너 환경
- Virtual Machines: 다중 노드 테스트

---

**참고**: 이 실습 가이드를 통해 kube-proxy의 세 가지 모드를 실제로 테스트하고 이해할 수 있습니다. 