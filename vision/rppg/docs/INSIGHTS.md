# 예시 코드로부터 인사이트

[`examples/offline_hr_from_video.py`](../examples/offline_hr_from_video.py)와 [rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox)([`README_TOOLBOX.md`](../examples/README_TOOLBOX.md) 절차)를 기준으로, 구현·실험 시 바로 쓸 만한 관찰을 정리했습니다.

## 1. 파이프라인 각 단계가 “가정”을 깔고 간다

1. **ROI**: Haar 얼굴 검출은 각도·가림에 약하고, 박스가 흔들리면 RGB 시계열에 **급격한 스텝 노이즈**가 섞입니다. “피부만”도 아니고 이마·볼 한정도 아닌 단순 박스라면 배경 색이 섞일 수 있습니다.
2. **시계열 샘플링**: OpenCV가 주는 FPS가 부정확하면 **주파수 축이 틀어져** BPM이 한꺼번에 어긋납니다. 긴 클립이면 프레임 타임스탬프로 재샘플링하는 편이 안전합니다.
3. **CHROM vs POS**: CHROM은 전 구간에 대해 채널 표준편차로 스케일을 맞추는 반면, POS는 **약 1.6초 윈도**마다 시간 정규화·투영을 반복합니다. 짧은 영상에서는 POS 윈도가 곧 전체 길이와 비슷해져 CHROM과 상관이 높아질 수 있습니다.
4. **대역통과**: 0.7–3.5 Hz(약 42–210 bpm)는 운동 심박까지 넓게 잡은 값입니다. 좁히면 SNR은 좋아질 수 있지만 고심박 구간에서 피크를 놓칩니다.
5. **Welch 피크 하나**: 하모닉(2배 주파수)이 더 큰 경우 오판이 납니다. **심박 하모닉 억제**나 **후보 피크 중 선택 규칙**이 실제 제품급과의 차이입니다.

## 2. 스크립트를 돌리며 확인할 체크리스트

- `--synthetic`으로 GREEN 경로가 대략 목표 BPM 근처로 나오는지 먼저 확인합니다(필터·Welch 경로 검증).
- 실제 영상에서 **GREEN / CHROM / POS** 세 값이 크게 갈리면, 조명 변화·얼굴 움직임·ROI 이탈·FPS 불확실성을 의심합니다.
- 얼굴이 잡히는 프레임 수가 적으면 경고 후 종료합니다. **정면·고정·충분한 길이**가 학습용에도 실전에도 필요합니다.

## 3. 다음 실습으로 이어가기 좋은 주제

1. **ROI 추적**: KLT, CSRT, 또는 MediaPipe Face Mesh로 피부 마스크 평균 RGB.
2. **윈도별 BPM**: 짧은 슬라이딩 윈도마다 BPM을 내어 **시간에 따른 심박 곡선** 만들기.
3. **공개 데이터 + 수치 평가**: [rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox)로 PURE/UBFC 등을 로드하고 **라벨 대 MAE/HR**를 재현하는 쪽이 가장 수월합니다(이 repo 예시는 라벨·데이터 로더 없음).
4. **EVM과의 연결**: 미세 색·움직임 증폭이 rPPG SNR에 어떤 영향을 주는지(다음 주제 `evm/`에서 비교).

## 4. 한 줄 요약

rPPG에서 알고리즘(CHROM/POS)만 바꿔서는 한계가 빨리 오고, **ROI 품질·타임베이스·하모닉**이 결과를 좌우합니다. 이 저장소의 예시는 그 “나머지 절반”을 건드리기 위한 출발점입니다.

## 5. rPPG-Toolbox (공식 `main.py` 흐름)

1. **역할 분리**: Toolbox는 “데이터 로더 + 전처리 + 여러 비지도/지도 알고리즘 + MAE/RMSE/MAPE/Pearson/SNR/Bland-Altman”까지 한 번에 돌리는 **실험 러너**에 가깝습니다. `offline_hr_from_video.py`는 ROI·필터·피크를 **직접 만져보며** 감을 잡는 용도로 유지하는 편이 좋습니다.
2. **설정이 곧 실험**: UBFC 비지도 YAML(`UBFC-rPPG_UNSUPERVISED.yaml`)은 `METHOD` 리스트로 **ICA, POS, CHROM, GREEN, …** 를 한 번에 평가합니다. 우리 스크립트는 세 가지(GREEN/CHROM/POS)만 Welch로 비교하므로, **지표 정의·윈도·FFT 평가**가 다르면 숫자가 1:1로 맞지 않는 것이 정상입니다.
3. **전처리 플래그**: Toolbox는 첫 실행 시 `DO_PREPROCESS: True`로 캐시를 만든 뒤, 이후에는 `False`로 두는 패턴이 README에 나와 있습니다. 캐시 경로(`CACHED_PATH`)를 잘못 잡으면 매번 느리거나 깨진 캐시를 읽습니다.
4. **ROI/백엔드 차이**: 기본 infer YAML은 얼굴 크롭(예: 72×72), Haar(`HC`) 또는 YOLO5Face(`Y5F`) 등 **고정된 전처리 파이프라인**을 탑니다. OpenCV Haar + 이마 밴드만 쓰는 예시 스크립트와는 **동일 조건이 아니므로** BPM이 어긋나도 “구현 버그”라기보다 파이프라인 차이일 가능성이 큽니다.
5. **경로·환경**: 업스트림 YAML의 `/gscratch/...` 예시 `DATA_PATH`는 반드시 고쳐야 합니다. 실행은 **Toolbox 전용 venv/conda**를 활성화한 뒤, 클론 루트에서 `python main.py --config_file ./configs/...` 가 가장 단순합니다(래퍼 불필요).

정리하면, **논문식 수치·여러 알고리즘 동시 비교**는 Toolbox로, **한 줄기 파이프라인 이해·빠른 가설 검증**은 `offline_hr_from_video.py`로 나누는 구성이 실무에서도 무난합니다.
