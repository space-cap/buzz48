# Cloudflare Tunnel (Zero Trust) 설정 가이드

본 문서는 오라클 클라우드(OCI) VM 인스턴스 전면에 **Cloudflare Tunnel (Zero Trust)**을 구축하여 인바운드 방화벽 포트(80, 443 등)를 완전히 폐쇄한 채 안전한 HTTPS/WSS 웹 서비스를 배포하는 방법을 다룹니다.

---

## 1. 아키텍처 및 보안상의 이점

```mermaid
graph LR
    User([유저 브라우저]) -->|HTTPS / WSS| CF[Cloudflare Edge]
    CF -.->|암호화 터널 (Outbound)| CFT[cloudflared 데몬]
    
    subgraph OCI Compute Instance (140.245.64.172)
        CFT -->|http://localhost:3001| NextJS[Next.js 프론트엔드]
        CFT -->|http://localhost:8082| GoAPI[Go API 서버]
        CFT -->|http://localhost:8081| GoWS[Go WebSocket 서버]
    end
```

* **인바운드 포트 전체 차단**: OCI 보안 규칙(Security List)에서 80(HTTP), 443(HTTPS) 포트를 외부에 열지 않아도 서비스가 가능합니다. 외부 해커의 포트 스캔 및 직접적인 IP 디도스 공격을 원천 봉쇄합니다.
* **SSL 인증서 완전 관리**: HTTPS 연결에 필요한 SSL/TLS 인증서 발급 및 자동 갱신을 Cloudflare 엣지 단에서 자동으로 처리합니다.
* **WebSocket 지원**: Cloudflare Tunnel은 별도 추가 설정 없이 WebSocket 프로토콜(WSS)의 업그레이드 요청을 내부적으로 자동 중계합니다.

---

## 2. 사전 필수 준비 사항

1. **Cloudflare 계정 및 도메인 연동**:
   * 소유하신 도메인(예: `pl3.kr`)의 네임서버가 Cloudflare로 지정되어 활성화되어 있어야 합니다.
2. **OCI VM 터미널 접속**:
   * SSH를 통해 오라클 클라우드 인스턴스(`140.245.64.172`)에 접속 가능한 상태여야 합니다.

---

## 3. Step-by-Step 설정 절차

### Step 1. Cloudflare 대시보드에서 터널 생성
1. **Cloudflare Zero Trust 대시보드**([https://one.dash.cloudflare.com/](https://one.dash.cloudflare.com/))에 접속합니다.
2. 좌측 메뉴에서 **Networks** ➡️ **Tunnels**를 선택한 후, **Add a tunnel**을 클릭합니다.
3. 터널 유형 선택에서 **cloudflared**를 선택하고 **Next**를 누릅니다.
4. 터널 이름(예: `buzz48-tunnel`)을 식별하기 쉽게 입력한 후 **Save tunnel**을 클릭합니다.

### Step 2. OCI 우분투 서버에 커넥터(`cloudflared`) 설치
1. 터널 생성이 완료되면 화면에 OS별 설치 명령어 가이드가 나타납니다.
2. **`Debian / Ubuntu` (64-bit)** 탭을 선택합니다.
3. 우측 박스에 제공되는 **설치 명령어 스크립트**를 전체 복사합니다.
   * *예시 스크립트 형태:*
     ```bash
     curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb && \
     sudo dpkg -i cloudflared.deb && \
     sudo cloudflared service install <토큰값>
     ```
4. OCI VM 터미널 창에 접속한 뒤 복사한 명령어를 붙여넣어 실행합니다.
5. 설치가 성공하면 Cloudflare 웹 페이지 하단에 연결 상태가 빨간색 `INACTIVE`에서 녹색 **`ACTIVE`**로 바뀌며 데몬이 등록되었음을 알려줍니다.

### Step 3. DNS 및 Public Hostname 라우팅 규칙 설정
터널 설정 페이지의 **Public Hostname** 탭으로 이동하여 **Add a public hostname**을 클릭하고 아래 3가지 규칙을 순서대로 추가합니다.

#### ① Next.js 프론트엔드 라우팅 규칙
* **Subdomain**: `buzz48`
* **Domain**: `pl3.kr` (결과 도메인: `buzz48.pl3.kr`)
* **Path**: *(비워둠)*
* **Service Type**: `HTTP`
* **URL**: `localhost:3001`

#### ② Go API 서버 라우팅 규칙 (`/v1` 하위 경로)
* **Subdomain**: `buzz48`
* **Domain**: `pl3.kr`
* **Path**: `v1` *(슬래시 없이 `v1` 입력)*
* **Service Type**: `HTTP`
* **URL**: `localhost:8082`

#### ③ Go WebSocket 서버 라우팅 규칙 (`/ws` 하위 경로)
* **Subdomain**: `buzz48`
* **Domain**: `pl3.kr`
* **Path**: `ws` *(슬래시 없이 `ws` 입력)*
* **Service Type**: `HTTP` *(WS 업그레이드 통신도 내부적으론 HTTP 형식을 거쳐 라우팅됩니다.)*
* **URL**: `localhost:8081`

---

## 4. 로컬 인스턴스 방화벽 및 동작 검증

### VM 내 포트 가동 상태 진단
데몬 설정 완료 후 서버 내부에서 실제 포트들이 열려있고 프로세스가 정상 대기 중인지 조회합니다.
```bash
# TCP 수신 포트 목록 및 PID 확인
sudo ss -tlnp | grep -E "3001|8081|8082"
```

### 브라우저 최종 접속 테스트
모든 파이프라인과 라우팅이 완료되면 웹 브라우저 주소창에 다음 주소를 입력하여 접속합니다.
* **접속 주소**: [https://buzz48.pl3.kr](https://buzz48.pl3.kr)
* **동작 점유**:
  * SSL 자물쇠 마크가 정상 활성화되는지 확인합니다.
  * API 서버 호출 및 실시간 WebSocket 채팅 연동이 매끄럽게 연결되는지 테스트합니다.
