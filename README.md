# IconStudio (IconStudioGo) 🎨✨

[![Version](https://img.shields.io/badge/version-2.2.4-blue.svg)](https://github.com/CtOS-Dev14/IconStudioGo)
[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8.svg)](https://golang.org)
[![Wails](https://img.shields.io/badge/wails-v2-red.svg)](https://wails.io)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%2F11%20%7C%20WinPE-0078D6.svg)](https://microsoft.com)

**IconStudioGo**는 Go 언어와 Wails v2 프레임워크 기반의 초경량·초고속 Windows 리소스 아이콘 추출/관리 도구입니다. 기존 스크립트 기반 구현의 성능 한계와 메모리 오버헤드를 극복하고, 단일 독립 실행형(Standalone) 바이너리로 개발되었습니다.

---

## 🚀 주요 기능 (Features)

### 1. 초고속 무손실 PE 파서 (Pure Go PE Parser)
- 32-bit (PE32) 및 64-bit (PE32+) Windows 바이너리(`.exe`, `.dll`, `.ocx`, `.cpl`, `.scr`, `.mun` 등)를 네이티브 수준에서 신속하게 파싱합니다.
- 외부 의존성 없이 순수 Go 구현으로 `RT_GROUP_ICON` 및 `RT_ICON` 리소스 트리를 직접 분석합니다.

### 2. 무손실 UHD 및 멀티 해상도 완벽 지원
- 256×256 UHD 고해상도(PNG 압축 포맷 포함) 아이콘을 압축 손실 없이 완벽하게 복원 및 렌더링합니다.
- 16×16, 24×24, 32×32, 48×48, 64×64, 128×128, 256×256 등 포함된 모든 서브 해상도 프레임을 분리 추출 및 일괄 내보내기 가능.

### 3. 직관적인 UI & Windows 11 Fluent 테마
- Windows 11 Mica/Acrylic 스타일의 미려하고 반응성 높은 사용자 인터페이스.
- OS 다크/라이트 테마 자동 동기화 및 수동 토글 지원.
- 왼쪽 리스트에서 고화질 썸네일 즉시 확인 및 상세 프리뷰 지원.

### 4. 드래그 앤 드롭 (Drag & Drop) 완벽 지원
- 탐색기에서 파일을 창 위로 드래그하면 즉시 리소스 분석 및 아이콘 목록 로딩.

### 5. 완벽한 Portable & WinPE 모드 지원
- 시스템 레지스트리나 `%APPDATA%`에 의존하지 않고, 실행 파일이 위치한 폴더에 `settings.json`을 저장합니다.
- USB 메모리, 외장 드라이브, Windows PE 환경에서도 이전 창 크기, 위치, 테마 설정이 그대로 유지됩니다.

### 6. 단일 실행 보장 (Single Instance Lock)
- 프로세스 중복 실행을 원천 차단하여 메모리와 리소스를 보호합니다.
- 프로그램이 실행 중일 때 추가 실행을 시도하면 기존 창이 자동으로 최소화 해제 및 최상위로 활성화(Bring to front)됩니다.
- 다른 파일을 통해 연결 실행 시 기존 창에서 해당 파일을 즉시 감지하여 자동으로 불러옵니다.

---

## 🛠️ 기술 스택 (Tech Stack)

- **Backend**: Go (Golang)
  - `pkg/peicon`: PE32/PE32+ 리소스 파서, ICO/PNG/BMP 스트림 디코더 및 패커
  - `pkg/settings`: WinPE 호환 실행 폴더 기준 영구 설정 매니저
- **Frontend**: Wails v2 (HTML5 / Modern Vanilla JS / Fluent CSS)
- **GUI Engine**: Microsoft Edge WebView2 (Chromium 기반 네이티브 임베딩)

---

## 📦 빌드 방법 (Build Instructions)

### 필수 요구사항
- [Go](https://golang.org/dl/) (1.24 이상 권장)
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- [Node.js](https://nodejs.org/) (프론트엔드 에셋 패키징용)

### 빌드 명령어
```powershell
# 프로젝트 클론
git clone https://github.com/CtOS-Dev14/IconStudioGo.git
cd IconStudioGo

# 프로덕션 단일 실행 파일 빌드 (build/bin/IconStudio_v2.2.4.exe 생성)
wails build -clean -nsis=false -o IconStudio_v2.2.4.exe
```

---

## 📝 버전 히스토리 (Changelog)

### v2.2.4 (Build 2.2.4.0)
- **단일 실행 보장 (Single Instance Lock)**: 중복 프로세스 실행 방지 및 기존 창 자동 활성화
- **스마트 인자 연동**: 이미 실행 중일 때 외부 파일 실행/연결 시 기존 창에서 즉시 로드

### v2.2.3 (Build 2.2.3.0)
- **Portable WinPE 모드**: 실행 파일 기준 로컬 설정 저장(`settings.json`) 적용
- **UHD 렌더링 최적화**: 256px 고화질 아이콘 썸네일 손실 없는 디코딩 적용
- **UI 개선**: 리스트 썸네일 확대 및 텍스트 레이블 간소화
- **창 상태 보존**: 창 위치(X, Y), 크기(Width, Height), 테마(Dark/Light) 영구 보존
- **글로벌 드래그 앤 드롭**: 탐색기 파일 드래그 자동 감지 안정화

---

## 📄 라이선스 (License)

This project is licensed under the MIT License.
