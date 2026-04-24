# rPPG 리서치 노트

카메라로 촬영한 피부 영역에서 **혈류에 따른 미세한 반사 스펙트럼·색도 변화**를 추출해 맥박(심박) 등을 비접촉으로 추정하는 기술입니다. PPG(광용적맥파)의 “원격(remote)” 버전으로 이해하면 됩니다.

## 핵심 개념

1. **광학적 근거**: 헤모글로빈에 의한 가시광 흡수·산란이 맥박 주기로 변하고, 특히 녹색 대역 근처에서 변화가 관측되기 쉽다는 보고가 많습니다. 동시에 **RGB 채널 간 상대 비율(색도)** 에도 맥박 성분이 나타납니다.
2. **간섭 요인**: 전역 조명 변화, 얼굴·카메라 상대 움직임, 압축 노이즈, 피부톤·화장품, 반사광(specular) 등이 **운동·조명 아티팩트**로 섞입니다.
3. **전형적 파이프라인**: (얼굴·피부) **ROI 추적** → 프레임마다 **공간 평균 RGB 시계열** → (선택) 전처리 → **BVP(혈관 맥박파) 추정 알고리즘** → **대역통과** → **주파수 피크 / 머신러닝**으로 심박 추정.

## 대표 알고리즘(고전 신호처리)

| 방법 | 아이디어 | 장단(요약) |
|------|-----------|------------|
| **GREEN** | 주로 G 채널 강도 시계열 사용 | 구현 단순, 조명·피부톤에 민감 |
| **CHROM** | RGB 선형 결합으로 색도 성분에서 맥박 강조 (De Haan & Jeanne, 2013) | 움직임에 상대적으로 강건한 편 |
| **POS** | 슬라이딩 윈도에서 시간 정규화 후, 피부 평면에 직교하는 투영으로 성분 분리 (Wang et al., 2016) | 조명·피부톤 변화에 비교적 관대, 구현 시 윈도·FPS 설계가 중요 |

문헌에 따르면 **딥러닝 기법**은 정제된 공개 데이터셋에서는 강하지만, **고심박·저조도·도메인 이동**에서는 전통적 POS/CHROM이 더 나은 경우도 보고됩니다(예: CHILL 데이터셋 평가).

## 공개 데이터셋·벤치마크(입문용)

- **PURE**, **UBFC-rPPG**, **COHFACE** 등: 알고리즘 비교·재현에 자주 인용됩니다.
- 최근 **CHILL** 등: 운동으로 심박이 올라간 조건·저조도 등 “어려운” 설정을 다룹니다.

## 참고 문헌·자료

1. De Haan, G., & Jeanne, V. (2013). *Robust pulse rate from chrominance-based rPPG.* IEEE TBME. ([IEEE Xplore](https://ieeexplore.ieee.org/document/6523147))
2. Wang, W., den Brinker, A. C., Stuijk, S., & de Haan, G. (2016). *Algorithmic principles of remote PPG.* IEEE TBME. ([IEEE Xplore](https://ieeexplore.ieee.org/document/7377089))
3. npj Digital Medicine (2025). *The reliability of remote photoplethysmography under low illumination and elevated heart rates* — CHILL 데이터셋 및 다알고리즘 비교. ([Nature](https://www.nature.com/articles/s41746-025-02192-y))
4. Frontiers in Bioengineering and Biotechnology (2024). *Deep learning and remote photoplethysmography…* — 서베·동향. ([Frontiers](https://www.frontiersin.org/journals/bioengineering-and-biotechnology/articles/10.3389/fbioe.2024.1420100/full))
5. 오픈소스 구현 참고: [phuselab/pyVHR](https://github.com/phuselab/pyVHR) — CHROM·POS 등 BVP 메서드 모음(`pyVHR/BVP/methods.py`).
6. **[ubicomplab/rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox)** (NeurIPS 2023 데모): **공개 데이터 로더, 전통·딥러닝 모델, 학습/평가 설정(YAML)** 까지 묶인 표준 실험 프레임워크입니다. 논문 수준 재현·벤치마크·신규 모델 학습에는 이쪽이 적합합니다.

### rPPG-Toolbox vs 이 저장소의 `examples/`

| | [rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox) | 이 repo의 `offline_hr_from_video.py` |
|---|------|------|
| 목적 | 데이터셋·모델·지표를 통일해 **연구 재현** | 알고리즘 한 줄기를 **읽고·고치기 쉬운** 최소 예시 |
| 스택 | PyTorch, 설정 파일, 여러 데이터 포맷 | NumPy, SciPy, OpenCV만 |
| 권장 시점 | PURE/UBFC 등으로 **MAE/HR 비교**, 딥모델 학습 | 파이프라인 감 잡기, 로직만 빠르게 실험 |

**정리**: Toolbox를 “사용 안 하는” 것이 아니라, **현재 예시 코드에는 번들하지 않았을 뿐**이고, 다음 단계(공식 데이터로 수치 평가·TS-CAN 등 학습)에서는 Toolbox를 쓰는 것을 권장합니다.

실행 절차 요약은 [`examples/README_TOOLBOX.md`](../examples/README_TOOLBOX.md)를 참고하세요(클론한 Toolbox 루트에서 공식대로 `main.py` 실행).

## 이 저장소 예시 코드의 범위

- OpenCV 얼굴 검출 + ROI 평균 RGB → **CHROM / POS** → **Butterworth 대역통과** → **Welch PSD 피크**로 BPM 추정 (`offline_hr_from_video.py`).
- 공개 데이터·공식 지표로의 확장은 **rPPG-Toolbox**를 클론한 뒤, 그 프로젝트 안에서 `main.py`와 YAML로 진행합니다.
- 얼 추적(tracking)·딥러닝·다중 ROI 투표는 포함하지 않습니다. 다음 단계 실습 주제로 남겨 두었습니다.
