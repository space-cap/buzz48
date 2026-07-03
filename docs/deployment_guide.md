# 오라클 클라우드(OCI) 배포 가이드

본 문서는 소유하신 도메인 `buzz48.pl3.kr`과 Cloudflare DNS, 그리고 오라클 클라우드(OCI) VM 인스턴스(`140.245.64.172`)를 활용하여 Buzz48 서비스의 백엔드, 프론트엔드, Redis, WebSocket을 실서비스 환경에 배포하는 종합 안내서입니다.

---

## 1. 실서비스 배포 아키텍처 개요

단일 도메인 `buzz48.pl3.kr` 아래에서 SSL(HTTPS/WSS) 인증서 및 CORS 이슈를 깔끔하게 해소하기 위해 **Nginx 리버스 프록시**를 전면에 배치합니다.

```mermaid
graph TD
    User([유저 브라우저]) -->|HTTPS / WSS| CF[Cloudflare CDN]
    CF -->|Port 443| Nginx[Nginx 리버스 프록시]
    
    subgraph OCI Compute Instance (140.245.64.172)
        Nginx -->|/ | NextJS[Next.js 프론트엔드 :3001]
        Nginx -->|/v1/*| GoAPI[Go API 서버 :8082]
        Nginx -->|/ws| GoWS[Go WebSocket 서버 :8081]
        
        GoAPI <--> Redis[(Redis :6379)]
        GoWS <--> Redis
    end
    
    GoAPI <--> NeonDB[(Neon PostgreSQL Cloud)]
```

- **데이터베이스**: 현재 사용 중인 Neon PostgreSQL Cloud DB를 그대로 연동하므로 OCI 내부에는 별도의 무거운 DB를 설치하지 않아 자원을 절약합니다.
- **Redis**: OCI 로컬에 초경량 Redis 서버를 직접 구동하여 동접자 및 조회수 가드용 인메모리 저장소로 씁니다.

---

## 2. 사전 인프라 설정

### 2.1 Cloudflare DNS 설정
1. Cloudflare 대시보드 ➡️ `pl3.kr` 도메인 관리 영역으로 이동합니다.
2. **DNS 레코드**에 아래 설정을 추가합니다:
   - **Type**: `A`
   - **Name**: `buzz48` (즉, `buzz48.pl3.kr`)
   - **IPv4 Address**: `140.245.64.172`
   - **Proxy status**: `Proxied` (오렌지 구름 활성화 - Cloudflare SSL/TLS 암호화 혜택 및 IP 숨김 지원)
3. **SSL/TLS 암호화 모드**:
   - Cloudflare SSL/TLS 탭에서 암호화 모드를 `Full` (권장, Nginx에 자체 인증서 적용) 또는 `Flexible`로 지정합니다.

### 2.2 오라클 클라우드(OCI) 방화벽 개방 (중요)
오라클 클라우드는 기본적으로 모든 포트가 닫혀 있으므로 **OCI 대시보드**와 **인스턴스 내부 방화벽**을 모두 열어주어야 접속이 가능합니다.

#### [Step 1] OCI Subnet 수신 규칙(Ingress Rules) 추가
1. OCI 콘솔 ➡️ Compute ➡️ Instances ➡️ 본인 인스턴스 클릭.
2. Primary VNIC 섹션의 **Virtual Cloud Network (VCN)** 링크 클릭 ➡️ **Security Lists** 클릭.
3. 기본 보안 리스트(Default Security List)에 아래의 **수신 규칙(Ingress Rule)** 2개를 추가합니다:
   - **소스 CIDR**: `0.0.0.0/0` (전체 개방)
   - **IP 프로토콜**: `TCP`
   - **대상 포트 범위**: `80` (HTTP)
   - *두 번째 규칙 추가*:
   - **소스 CIDR**: `0.0.0.0/0`
   - **IP 프로토콜**: `TCP`
   - **대상 포트 범위**: `443` (HTTPS)

#### [Step 2] 인스턴스 Ubuntu OS 내 방화벽 개방
SSH로 OCI 인스턴스에 접속한 뒤 아래 명령어로 포트를 개방하고 방화벽을 적용합니다:
```bash
# Ubuntu iptables 규칙 개방 (OCI 기본 방화벽 규칙 우회)
sudo iptables -I INPUT 6 -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -p tcp --dport 443 -j ACCEPT

# 영구 저장
sudo netfilter-persistent save
sudo netfilter-persistent reload
```

---

## 3. 서버 내부 환경 설정 및 패키지 설치

### 3.1 필요 도구 설치 (Git, Go, Node.js, Redis, Nginx)
SSH 접속 상태에서 아래 커맨드를 순서대로 실행합니다:
```bash
sudo apt update && sudo apt upgrade -y

# 1. Redis 설치 및 활성화
sudo apt install redis-server -y
sudo systemctl enable redis-server
sudo systemctl start redis-server

# 2. Nginx 설치 및 활성화
sudo apt install nginx -y
sudo systemctl enable nginx
sudo systemctl start nginx

# 3. Node.js & npm 설치 (Next.js 빌드 및 구동용)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs

# 4. Go 설치 (백엔드 컴파일용)
sudo snap install go --classic
```

---

## 4. 빌드 및 배포 자동화 프로세스

### 4.1 프로젝트 클론 및 환경변수(`.env`) 설정
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

서버가 재부팅되거나 에러로 프로세스가 죽었을 때 자동으로 되살아나도록 백엔드 서버(API, WS, Worker)와 프론트엔드를 **systemd 서비스**로 등록합니다.

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

## 6. Nginx 리버스 프록시 및 SSL 연동

### 6.1 Nginx 설정 구성 (`/etc/nginx/sites-available/buzz48`)
기본 설정을 날리고 아래 템플릿을 적용하여 HTTP(80) 요청을 3001포트(Next.js), `/v1/` 경로는 API서버, `/ws` 경로는 WebSocket 서버로 포워딩합니다.

```nginx
server {
    listen 80;
    server_name buzz48.pl3.kr;

    # Certbot SSL 적용 전 임시 설정
    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # Go API 프록시
    location /v1/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Go WebSocket 프록시 (WSS 자동 지원용 프로토콜 업그레이드 헤더 필수)
    location /ws {
        proxy_pass http://127.0.0.1:8081;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
```

```bash
# 설정 활성화 및 Nginx 재시작
sudo ln -s /etc/nginx/sites-available/buzz48 /etc/nginx/sites-enabled/
sudo rm /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl restart nginx
```

### 6.2 Certbot을 통한 SSL(HTTPS) 인증서 적용
Nginx 도메인 연결이 완료되면 Let's Encrypt 무료 SSL 인증서를 발급받아 원클릭으로 주입합니다:
```bash
sudo apt install certbot python3-certbot-nginx -y
sudo certbot --nginx -d buzz48.pl3.kr
```
인증서 발급 질문 단계에서 **Redirect HTTP requests to HTTPS**를 선택하면, 80포트 접속을 자동으로 443(HTTPS) 안전 접속으로 변경해 주는 보안 룰까지 Nginx에 자동 완성해 줍니다!
이제 브라우저에서 `https://buzz48.pl3.kr` 에 접속하면 자물쇠 표시와 함께 실시간 채팅 및 방문 화력 카운팅 서비스가 완벽하게 배포 가동됩니다!
