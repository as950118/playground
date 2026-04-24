# rPPG (Remote Photoplethysmography) 실습

얼굴 등 피부 영역의 **미세한 색 변화**로부터 맥박·심박수를 추정하는 비접촉 PPG 파이프라인을 연습합니다.

## 이 폴더에서 하는 일

1. [`docs/RESEARCH.md`](./docs/RESEARCH.md) — 개념, 대표 알고리즘, 데이터셋·문헌 링크( **[rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox)** 와 이 예시의 역할 구분 포함)
2. [`examples/offline_hr_from_video.py`](./examples/offline_hr_from_video.py) — ROI RGB 추출 → CHROM / POS → 대역통과 → Welch 피크로 심박 추정
3. [`examples/README_TOOLBOX.md`](./examples/README_TOOLBOX.md) — 클론 후 **UBFC-rPPG + 비지도 YAML** 한 경로로 첫 실행까지 안내  
4. (선택) [`scripts/patch_ubfc_unsupervised_yaml.py`](./scripts/patch_ubfc_unsupervised_yaml.py) — Toolbox YAML 경로 일괄 치환
5. [`docs/INSIGHTS.md`](./docs/INSIGHTS.md) — 위를 기준으로 한 설계·한계·개선 방향

연구용 통합 환경은 별도 클론으로 [ubicomplab/rPPG-Toolbox](https://github.com/ubicomplab/rPPG-Toolbox)를 두고, 이 폴더 예시는 “최소 파이프라인 이해용”으로 병행하면 됩니다.

## 빠른 시작

```bash
cd rppg
python -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python examples/offline_hr_from_video.py --help
```

rPPG-Toolbox는 별도 클론 후 [`examples/README_TOOLBOX.md`](./examples/README_TOOLBOX.md)대로 **Toolbox 루트에서** `python main.py --config_file ...` 를 실행하면 됩니다.

얼굴이 잘 보이는 짧은 클립(정면, 조명 안정)으로 시도하세요.

```bash
python examples/offline_hr_from_video.py --video /path/to/clip.mp4
```

웹캠이 있으면:

```bash
python examples/offline_hr_from_video.py --camera 0 --seconds 15
```

## 주의

- 이 코드는 **교육용**이며, 의료기기나 임상 지표로 사용할 수 없습니다.
- 심박 추정 오차는 조명, 움직임, 피부톤, 압축 아티팩트, FPS에 크게 좌우됩니다.
