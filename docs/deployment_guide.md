# 오라클 클라우드(OCI) 배포 가이드 (Cloudflare Tunnel 기반)

본 문서는 소유하신 도메인 `buzz48.pl3.kr`과 Cloudflare DNS, 그리고 오라클 클라우드(OCI) VM 인스턴스(`140.245.64.172`)를 활용하여 **Cloudflare Tunnel (Zero Trust)** 기술로 배포하는 전용 안내서입니다.

---

## 1. Cloudflare Tunnel 배포 아키텍처 개요

**Cloudflare Tunnel** 방식을 도입하면 OCI 서버 전면에 무겁고 복잡한 Nginx를 설치할 필요가 전혀 없으며, OCI 인바운드 방화벽 포트(80, 443 등)를 **단 하나도 열지 않고 완전 폐쇄한 상태**에서도 세계 최고 수준의 HTTPS/WSS 보안 연결을 구현할 수 있습니다.

```mermaid
graph TD
    User([유저 브라우저]) -->|HTTPS / WSS| CF[Cloudflare Edge]
    CF -.->|안전한 암호화 터널| CFT[cloudflared 데몬]
    
    subgraph OCI Compute Instance (140.245.64.172)
        CFT -->|http://localhost:3001| NextJS[Next.js 프론트엔드 :3001]
        CFT -->|http://localhost:8082| GoAPI[Go API 서버 :8082]
        CFT -->|http://localhost:8081| GoWS[Go WebSocket 서버 :8081]
        
        GoAPI <--> Redis[(Redis :6379)]
        GoWS <--> Redis
    end
    
    GoAPI <--> NeonDB[(Neon PostgreSQL Cloud)]
```

* **보안성 극대화**: 퍼블릭 포트를 전부 닫아두기 때문에 포트 스캔이나 디도스(DDoS) 공격 시도가 OCI VM 서버 본진에 도달조차 하지 못합니다.
* **인증서 관리 제로**: SSL/TLS 인증서 발급 및 주기적 갱신(Let's Encrypt 등)을 Cloudflare가 종단에서 자동 처리합니다.

---

## 2. 사전 환경 설정

### 2.1 OCI 방화벽 정책 (수신 규칙 없음)
- Cloudflare Tunnel은 OCI VM 내부에서 Cloudflare 서버로 **아웃바운드 터널**을 뚫어 나가는 원리입니다.
- 따라서 **OCI Subnet Security List(보안 리스트) 수신 규칙에서 HTTP(80)나 HTTPS(443) 포트를 전혀 열지 않아도 됩니다.** (기본 상태 그대로 전체 차단 유지 권장)

---

## 3. 서버 내부 환경 설정 및 패키지 설치

### 3.1 [사전 진단] 우분투 사용 중인 포트 충돌 확인
인스턴스에 이미 구동 중인 다른 서비스가 있을 수 있으므로, 본 프로젝트가 점유할 포트들(**`3001`**, **`8081`**, **`8082`**, **`6379`**)이 이미 점유되고 있는지 사전 진단합니다.

#### ① 현재 활성화된 모든 포트 및 프로세스 확인
```bash
# TCP 수신 대기(LISTEN) 상태의 모든 포트와 PID/프로세스명 조회
sudo ss -tlnp
```
*출력 결과의 `Local Address:Port` 컬럼에 우리가 사용할 포트 번호가 들어가 있는지 눈으로 체크합니다.*

#### ② 특정 포트를 사용 중인 프로세스 정밀 타격 조회
```bash
# 3001 포트 확인 예시
sudo lsof -i :3001
```

#### ③ 충돌 시 해결 방안
만약 기존의 불필요한 좀비 프로세스가 포트를 잡고 있다면 이를 즉시 강제 종료시킬 수 있습니다:
```bash
# 특정 포트를 점유한 프로세스 즉시 종료
sudo fuser -k 3001/tcp

# 또는 PID(Process ID)를 확인하여 수동 강제 종료
sudo kill -9 <PID>
```
*만약 기존의 중요 서비스가 해당 포트를 합법적으로 계속 써야 하는 경우라면, 프로젝트 환경 설정(`.env` 및 `.service` 유닛 파일) 내 포트 번호를 `3002`, `8085` 등으로 변경하여 충돌을 피할 수 있습니다.*

### 3.2 필요 도구 설치 (Git, Go, Node.js)
SSH 접속 상태에서 아래 커맨드를 실행하여 컴파일 및 런타임 도구를 설치합니다:
```bash
sudo apt update && sudo apt upgrade -y

# 1. Node.js & npm 설치 (Next.js 빌드 및 구동용)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# 2. Go 설치 (백엔드 컴파일용)
sudo snap install go --classic
```
*참고: Redis는 이미 서버 내에 Docker 컨테이너 형태로 6379 포트를 선점하여 성공적으로 구동 중이므로, 별도의 로컬 패키지 설치 단계는 생략하고 기존 Docker 컨테이너 인스턴스를 재사용합니다.*

---

## 4. 빌드 및 배포 프로세스

### 4.1 프로젝트 클론 및 환경변수(`.env`) 구성
서버 내 적절한 경로(예: `/var/www/buzz48`)에 프로젝트를 내려받고 `.env` 파일을 구성합니다.

```bash
sudo mkdir -p /var/www/buzz48
sudo chown -R $USER:$USER /var/www/buzz48
cd /var/www/buzz48

# 레포지토리 클론
git clone <본인_깃허브_레포지토리_주소> .
```

#### 백엔드용 `.env` 생성 (`/var/www/buzz48/backend/.env`)
```env
DB_HOST=ep-young-breeze-aoz29ou7.c-2.ap-southeast-1.aws.neon.tech
DB_PORT=5432
DB_NAME=neondb
DB_USER=neondb_owner
DB_PASSWORD=npg_vpI7DBm0oRew
DB_SSL_MODE=require

REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=

PORT=8082
WS_PORT=8081
```

#### 프론트엔드용 `.env.production` 생성 (`/var/www/buzz48/frontend/.env.production`)
```env
# 단일 도메인 구조이므로 API와 WS 주소를 동일 도메인의 하위 경로로 바라봅니다.
NEXT_PUBLIC_API_URL=https://buzz48.pl3.kr
NEXT_PUBLIC_WS_URL=wss://buzz48.pl3.kr/ws
```

### 4.2 빌드 프로세스
```bash
# 1. Go 백엔드 빌드
cd /var/www/buzz48/backend
go build -o api-server api/main.go
go build -o ws-server ws/main.go
go build -o worker-server cmd/worker/main.go

# 2. Next.js 프론트엔드 빌드
cd /var/www/buzz48/frontend
npm install
npm run build
```

---

## 5. 프로세스 상시 구동 설정 (Systemd Service)

서버 재부팅 시 자동으로 백엔드 및 프론트엔드 서버가 켜지도록 **systemd 서비스**로 등록합니다.

### 5.1 Go API 서버 등록 (`/etc/systemd/system/buzz-api.service`)
```ini
[Unit]
Description=Buzz48 Go API Server
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/var/www/buzz48/backend
ExecStart=/var/www/buzz48/backend/api-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 5.2 Go WebSocket 서버 등록 (`/etc/systemd/system/buzz-ws.service`)
```ini
[Unit]
Description=Buzz48 Go WebSocket Server
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/var/www/buzz48/backend
ExecStart=/var/www/buzz48/backend/ws-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 5.3 Go 배치 워커 등록 (`/etc/systemd/system/buzz-worker.service`)
```ini
[Unit]
Description=Buzz48 Go Batch Worker
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/var/www/buzz48/backend
ExecStart=/var/www/buzz48/backend/worker-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### 5.4 Next.js 프론트엔드 등록 (`/etc/systemd/system/buzz-frontend.service`)
```ini
[Unit]
Description=Buzz48 Next.js Frontend Server
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/var/www/buzz48/frontend
ExecStart=/usr/bin/npm run start -- -p 3001
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

#### 서비스 시작 및 상시 가동 활성화
```bash
sudo systemctl daemon-reload

sudo systemctl enable --now buzz-api
sudo systemctl enable --now buzz-ws
sudo systemctl enable --now buzz-worker
sudo systemctl enable --now buzz-frontend
```

---

## 6. Cloudflare Tunnel(Zero Trust) 연동 및 구동

Cloudflare Zero Trust 웹 관리자 화면에서 클릭 몇 번으로 터널을 구성하고 라우팅 설정을 완료합니다.

### 6.1 Cloudflare 대시보드 내 터널 생성
1. **Cloudflare Zero Trust 대시보드**([https://one.dash.cloudflare.com/](https://one.dash.cloudflare.com/))에 접속합니다.
2. 좌측 메뉴 ➡️ **Networks** ➡️ **Tunnels**를 선택하고, **Add a tunnel**을 클릭합니다.
3. 터널 이름(예: `buzz48-tunnel`)을 입력한 뒤 **Save tunnel**을 누릅니다.
4. OCI OS 환경인 **`Debian / Ubuntu` (64-bit)** 탭을 선택합니다.
5. 화면에 노출되는 설치 스크립트 커맨드(예: `curl -L ...` 및 `sudo cloudflared service install ...`)를 **전체 복사**하여 OCI VM 터미널에 붙여넣고 실행합니다.
   - *팁: 터미널에 성공적으로 명령어가 기동되면 대시보드 하단에 Status가 `ACTIVE` 로 실시간 감지되어 녹색 불이 켜집니다.*

### 6.2 도메인 및 하위 경로(Subpath) 라우팅 설정
터널 상세 설정 화면 우측 상단의 **Public Hostname** 탭을 눌러 아래 3개의 라우팅 규칙을 순서대로 추가(`Add a public hostname`)합니다.

#### 규칙 1: Next.js 프론트엔드 연동
- **Subdomain**: `buzz48`
- **Domain**: `pl3.kr` (즉, `buzz48.pl3.kr`)
- **Path**: *(비워둠)*
- **Service Type**: `HTTP`
- **URL**: `localhost:3001`

#### 규칙 2: Go API 서버 연동
- **Subdomain**: `buzz48`
- **Domain**: `pl3.kr`
- **Path**: `v1`  *(반드시 `/` 없이 `v1` 입력)*
- **Service Type**: `HTTP`
- **URL**: `localhost:8082`

#### 규칙 3: Go WebSocket 서버 연동
- **Subdomain**: `buzz48`
- **Domain**: `pl3.kr`
- **Path**: `ws`  *(반드시 `ws` 입력)*
- **Service Type**: `HTTP`  *(Cloudflare Tunnel은 내부적으로 ws/wss 업그레이드 프로토콜을 HTTP 타입을 통해 자동 지원합니다)*
- **URL**: `localhost:8081`

---

## 7. 가동 검증 및 동작 확인

모든 설정이 끝나면 브라우저 주소창에 **[https://buzz48.pl3.kr](https://buzz48.pl3.kr)** 을 입력하여 접속합니다.

* Cloudflare 엣지 브라우저단에서 HTTPS가 완전 관리되고 있으므로 별도의 SSL 경고창 없이 완벽한 자물쇠 표시가 나타납니다.
* 실시간 채팅 및 조회수가 문제없이 연동되어 움직이면 정상적으로 배포 배치가 종결된 것입니다.
