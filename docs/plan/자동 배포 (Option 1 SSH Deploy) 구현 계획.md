# 자동 배포 (Option 1: SSH Deploy) 구현 계획

본 계획서는 GitHub Actions와 SSH/SCP 전송을 이용하여 오라클 클라우드 VM(`140.245.64.172`)에 Buzz48 서비스(Go API, WebSocket, Worker, Next.js)를 자동으로 빌드 및 배포하는 파이프라인 구축을 목적으로 합니다.

---

## 📢 사용자 확인 및 준비 사항

이 자동 배포 파이프라인이 정상 작동하려면 GitHub 저장소의 **Settings > Secrets and variables > Actions** 메뉴에 아래의 **Repository Secrets**를 등록하셔야 합니다.

| Secret 이름 | 설명 | 예시 값 |
| :--- | :--- | :--- |
| `SSH_PRIVATE_KEY` | OCI VM에 접속할 수 있는 SSH 개인키 | `-----BEGIN OPENSSH PRIVATE KEY----- ...` |
| `DEPLOY_HOST` | OCI VM의 Public IP (보안 노출 방지) | `140.245.64.172` |
| `DEPLOY_USER` | OCI VM의 SSH 사용자 계정명 | `ubuntu` |

> [!IMPORTANT]
> **CPU 아키텍처 확인**:
> 사용 중이신 오라클 클라우드 인스턴스가 **AMD (1 Core, 1GB RAM)**인지 **ARM Ampere**인지에 따라 Go 컴파일 대상 아키텍처(`GOARCH`)가 달라집니다. 본 계획서의 워크플로우에서는 기본값으로 AMD 기준인 `GOARCH=amd64`로 작성되며, 만약 ARM 인스턴스인 경우 `GOARCH=arm64`로 수정이 필요합니다.

---

## 🛠️ 변경 예정 작업

### 1. 프론트엔드 최적화 (Next.js Standalone 빌드 활성화)
Next.js 빌드 결과물 크기를 압축하여 서버 메모리 점유 및 전송 시간을 단축합니다.

#### [MODIFY] [next.config.ts](file:///h:/lee/buzz48/frontend/next.config.ts)
* `output: "standalone"` 설정을 추가하여 독자적인 실행 파일과 필수 파일만 출력하도록 설정합니다.

---

### 2. GitHub Actions 워크플로우 추가

#### [NEW] [.github/workflows/deploy.yml](file:///h:/lee/buzz48/.github/workflows/deploy.yml)
* GitHub 리포지토리의 `main` 브랜치에 `push` 또는 `PR merge`가 발생할 때 자동으로 작동하는 CI/CD 워크플로우를 정의합니다.
* **주요 작업**:
  1. Go 런타임 설치 및 Go 바이너리 3개 컴파일 (`api-server`, `ws-server`, `worker-server`)
  2. Node.js 런타임 설치 및 Next.js 독립형 빌드 (`npm run build`)
  3. 빌드 결과물 패키징 (`tar.gz`)
  4. OCI VM에 SSH/SCP로 전송 및 압축 해제 (Secret에 등록된 DEPLOY_HOST, DEPLOY_USER 활용)
  5. 기존에 구동 중인 systemd 서비스 종료 ➡️ 새 파일 덮어쓰기 ➡️ 서비스 재기동 (`systemctl restart`)

---

## 🧪 검증 계획

### 1. 로컬 및 파일 생성 검증
* Next.js가 정상적으로 `standalone` 빌드가 완료되는지 로컬 컴파일을 시도합니다.
* 워크플로우 파일의 문법적 오류가 없는지 검증합니다.

### 2. 수동 검증 (배포 완료 후)
* 배포가 완료된 후, OCI VM에 접속하여 서비스 상태를 확인합니다:
  ```bash
  sudo systemctl status buzz-api buzz-ws buzz-worker buzz-frontend
  ```
* 브라우저에서 `https://buzz48.pl3.kr`에 접속하여 API 및 실시간 WebSocket이 올바르게 연결되는지 테스트합니다.
