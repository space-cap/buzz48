# 자동 배포 (CI/CD) 오류 해결 이력 및 가이드

본 문서는 오라클 클라우드(OCI) 1 CPU, 1GB RAM Ubuntu VM 환경에 Buzz48 서비스(Go API, WebSocket, Worker, Next.js) 자동 배포 파이프라인을 구축하면서 발생했던 주요 오류들과 원인, 그리고 해결 방법을 기록한 문서입니다.

---

## 📌 전체 배포 요약
* **방식**: GitHub Actions ➡️ SSH/SCP 전송 ➡️ systemd 서비스 구동
* **구조**:
  * **Next.js Frontend**: 포트 `3001` (Node Standalone 모드로 최적화 실행)
  * **Go REST API**: 포트 `8082` (Fiber 프레임워크)
  * **Go WebSocket**: 포트 `8081` (실시간 통신)
  * **Go 배치 워커**: 백그라운드 구동 (상시 실행)

---

## 🛠️ 발생 오류 및 해결 방법

### 1. SSH 인증 실패 (Handshake Failed)
#### ❌ 오류 현상
```text
drone-scp error: error copy file to dest: ***, 
error message: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey]
```
#### 🔍 원인
* 로컬 컴퓨터(Windows)에서 OCI VM 개인 키(`.key`) 파일을 열어 GitHub Secrets(`SSH_PRIVATE_KEY`)로 복사하는 과정에서 **줄바꿈 문자(CRLF/LF) 및 공백 문자가 깨지거나 유실**되어 SSH 키 파서가 이를 올바른 키로 인식하지 못함.
#### 💡 해결 방법
* Windows PowerShell에서 줄바꿈 형식의 유실 없이 안전하게 클립보드에 키 내용을 복사하는 명령어를 사용하여 GitHub Secrets에 재등록.
  ```powershell
  Get-Content H:\ssh-key-2026-06-19.key | Set-Clipboard
  ```
* 비밀키 내용 가장 마지막 줄(`-----END ...-----`)에 엔터(Enter)를 한 번 쳐서 **빈 줄을 하나 추가**하여 최종 저장.

---

### 2. SCP 파일 복사 권한 오류 (Permission Denied)
#### ❌ 오류 현상
```text
create folder /var/www/buzz48/tmp
drone-scp error: Process exited with status 1
```
#### 🔍 원인
* 리눅스 서버 내 `/var/www` 경로는 기본적으로 `root` 계정의 소유이므로, SSH 접속 계정인 `ubuntu` 유저가 폴더를 생성하거나 파일을 복사하려 할 때 쓰기 권한 부족으로 실패함.
#### 💡 해결 방법
* OCI 서버에 접속하여 배포 대상 디렉토리를 수동 생성하고 소유권을 `ubuntu` 유저로 이전.
  ```bash
  sudo mkdir -p /var/www/buzz48
  sudo chown -R ubuntu:ubuntu /var/www/buzz48
  ```

---

### 3. systemd 서비스 유닛 파일 미등록
#### ❌ 오류 현상
```text
Failed to stop buzz-api.service: Unit buzz-api.service not loaded.
Failed to start buzz-api.service: Unit buzz-api.service not found.
Process exited with status 5
```
#### 🔍 원인
* GitHub Actions 배포 스크립트가 배포 성공 후 서버에서 `systemctl restart buzz-api` 등을 호출했으나, OCI VM에 해당 서비스들을 백그라운드 구동하기 위한 **systemd `.service` 파일들이 사전에 등록되어 있지 않았음.**
#### 💡 해결 방법
* VM에 직접 접속하여 4개의 서비스 유닛 파일(`/etc/systemd/system/` 하위)을 생성하고 활성화.
  ```bash
  # systemd 리로드 및 활성화
  sudo systemctl daemon-reload
  sudo systemctl enable buzz-api buzz-ws buzz-worker buzz-frontend
  ```

---

### 4. Next.js 빌드 디렉토리(`.next`) 누락 오류
#### ❌ 오류 현상
```text
node[247048]: Error: Could not find a production build in the './.next' directory.
buzz-frontend.service: Main process exited, code=exited, status=1/FAILURE
```
#### 🔍 원인
* 기존 워크플로우 내 빌드 결과물 복사 명령이 `cp -r source/* target/` 형태였음.
* 리눅스 쉘에서 와일드카드 `*`는 **점(`.`)으로 시작하는 숨겨진 파일 및 폴더(Dotfiles)를 복사 범위에서 제외**시킴. 
* 그 결과 가장 핵심 구동 코드인 `.next` 빌드 폴더가 통째로 빠진 채 배포되어 실행 시 에러 발생.
#### 💡 해결 방법
* GitHub Actions 및 배포 스크립트 내부의 폴더 복사 명령에서 `*` 대신 `.`을 사용하여 점파일까지 포함하여 복사하도록 변경.
  * **수정 전**: `cp -r frontend/.next/standalone/* deploy/frontend/`
  * **수정 후**: `cp -r frontend/.next/standalone/. deploy/frontend/`

---

### 5. `ss` 포트 상태 조회 시 웹소켓 포트(`8081`) 누락 현상
#### ❌ 오류 현상
* 웹소켓 서버가 정상 실행(`🚀 WebSocket 서버 시작: ws://localhost:8081`)되었는데도 `sudo ss -tulp | grep :8081` 조회 시 결과가 나타나지 않음.
#### 🔍 원인
* 리눅스의 네트워크 조회 도구인 `ss`는 `/etc/services` 파일을 참고하여 알려진 포트 번호를 이름(Service Name)으로 매핑하여 출력함.
* `8081` 포트는 시스템 상에서 `tproxy` 또는 `blackice-icecap`으로 변환되어 출력되었기 때문에 `:8081` 검색어 패턴 매칭에서 누락됨.
#### 💡 해결 방법
* 포트 번호를 이름이 아닌 숫자로 강제 표시하게 하는 **`-n` (numeric) 옵션**을 추가하여 포트 상태 확인.
  ```bash
  sudo ss -tlnp | grep -E ':3001|:8081|:8082'
  ```
