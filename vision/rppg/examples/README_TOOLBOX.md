# rPPG-Toolbox: 클론 후 첫 실행 가이드

여기서는 **한 가지 경로만** 안내합니다: **UBFC-rPPG** 데이터 + **`UBFC-rPPG_UNSUPERVISED.yaml`** (ICA, POS, CHROM, GREEN 등 비지도 알고리즘 일괄 평가).

공식 저장소: [ubicomplab/rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox)

---

## 0. 전제

- `rPPG-Toolbox`를 이미 클론해 두었다고 가정합니다. 아래에서 그 경로를 `RPPG_TB`라고 부릅니다.
- **UBFC-rPPG**를 내려받을 수 있어야 합니다. [데이터 페이지](https://sites.google.com/view/ybenezeth/ubfcrppg)에서 신청·다운로드 후, 폴더 구조는 다음과 같아야 합니다 (`DATA_PATH`는 **`subject1`의 부모**).

```text
$RPPG_DATA/UBFC-rPPG/
  subject1/vid.avi
  subject1/ground_truth.txt
  subject2/...
```

`DATA_PATH`에는 `.../UBFC-rPPG` 까지 넣으면 됩니다 (그 안에 `subject*`가 있는 디렉터리).

---

## 1. Python 환경

**Anaconda(miniconda)는 필수가 아닙니다.** 공식 저장소는 `conda`와 `uv`를 예시로 들 뿐이고, **`python -m venv`로 만든 가상환경**에도 동일하게 설치하면 됩니다. (공식 `bash setup.sh uv`도 결국 `.venv`를 만듭니다.)

### venv로 설치 (conda 없이)

Toolbox는 Python **3.8** 전후를 많이 씁니다. 클론 루트에서:

```bash
cd "$RPPG_TB"
python3.8 -m venv .venv          # 3.8이 없으면 3.9 등으로 시도 후 requirements 호환 확인
source .venv/bin/activate        # Windows: .venv\Scripts\activate
pip install -U pip setuptools wheel
```

그다음 **본인 머신에 맞는 PyTorch**를 먼저 깝니다.

- **Linux + NVIDIA**: [PyTorch 시작 페이지](https://pytorch.org/get-started/locally/)에서 CUDA 조합에 맞는 `pip install torch ...` 명령 사용 (공식 `setup.sh conda`는 CUDA 12.1 휠을 고정으로 깔아 둠).
- **macOS / CPU만**: 같은 페이지에서 CPU용 휠로 설치.

이어서:

```bash
cd "$RPPG_TB"
pip install -r requirements.txt
cd tools/mamba && python setup.py install && cd ../..
```

`mamba` 빌드가 막히면 Toolbox 이슈/문서를 참고하세요.

### (선택) conda로 공식 스크립트 쓰기

클론 루트에서:

```bash
cd "$RPPG_TB"
bash setup.sh conda
conda activate rppg-toolbox
```

`setup.sh conda`는 **CUDA 12.1**용 PyTorch를 깔도록 되어 있어, GPU가 없거나 맥이면 실패할 수 있습니다. 그때는 위 **venv** 절에서 CPU(또는 자신의 CUDA)용 `torch`를 직접 맞추면 됩니다.

### 동작 확인

```bash
cd "$RPPG_TB"
python -c "import torch; print('torch', torch.__version__)"
```

---

## 2. YAML 경로 맞추기 (두 가지 중 하나)

대상 파일: `$RPPG_TB/configs/infer_configs/UBFC-rPPG_UNSUPERVISED.yaml`

### 방법 A — 수동 편집

다음 키만 본인 환경에 맞게 수정합니다.

| 키 | 의미 |
|----|------|
| `UNSUPERVISED.DATA.DATA_PATH` | UBFC-rPPG 루트 (`subject1` 상위) |
| `UNSUPERVISED.DATA.CACHED_PATH` | 전처리 캐시를 둘 빈 디렉터리 (미리 만들어 두면 좋음) |
| `UNSUPERVISED.DATA.DO_PREPROCESS` | **첫 실행**: `True` → 전처리 끝나면 `False`로 바꿈 |

기본값에 있는 `/gscratch/...` 는 예시라 그대로 두면 실패합니다.

### 방법 B — practice-vision 스크립트로 치환

`practice-vision`의 `rppg` 폴더에서 (경로만 예시):

```bash
cd /path/to/practice-vision/rppg

python3 scripts/patch_ubfc_unsupervised_yaml.py \
  --toolbox-root "$RPPG_TB" \
  --data-path "$RPPG_DATA/UBFC-rPPG" \
  --cache-path "$RPPG_DATA/rppg_toolbox_cache" \
  --set-do-preprocess true
```

전처리가 끝난 뒤 같은 명령에 `--set-do-preprocess false`만 바꿔 다시 실행하면 됩니다.

---

## 3. 실행 명령 (이 한 줄이 본론)

Toolbox **클론 루트**에서, 위에서 만든 환경을 켠 상태로:

```bash
cd "$RPPG_TB"
python main.py --config_file ./configs/infer_configs/UBFC-rPPG_UNSUPERVISED.yaml
```

성공하면 설정이 출력되고, YAML에 적힌 `METHOD` 목록에 대해 순차 평가가 돌아갑니다. 로그·그래프는 YAML의 `LOG.PATH`(기본값은 공식 README의 `runs/exp` 계열) 아래에 쌓입니다.

---

## 4. 자주 막히는 지점

- **`DATA_PATH`**: `subject1`이 바로 아래에 없으면 로더가 실패합니다.
- **`DO_PREPROCESS`**: 첫 실행이 아닌데 `True`이면 전처리를 다시 돌립니다. 캐시를 채운 뒤에는 `False`가 맞습니다.
- **GPU / Mac**: CUDA 휠이 깔린 환경에서 CPU만 있으면 import 단계에서 막힐 수 있습니다. 해당 환경에 맞는 `torch`로 다시 설치하세요.

---

## 5. 다음에 시도할 설정 (참고)

지도 학습 모델 추론 예 (가중치·데이터 경로를 YAML에 맞춘 뒤):

```bash
python main.py --config_file ./configs/infer_configs/PURE_UBFC-rPPG_TSCAN_BASIC.yaml
```

전체 옵션·키 설명은 [Toolbox README](https://github.com/ubicomplab/rPPG-Toolbox/blob/main/README.md)의 YAML 절을 따릅니다.
