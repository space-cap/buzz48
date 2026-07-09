# Cloudflare Origin Rules 설정 가이드 (대체 방식)

본 문서는 오라클 클라우드(OCI) VM 내부에 추가 프로그램(`cloudflared` 등)을 설치하지 않고, **Cloudflare DNS Proxy**와 **Origin Rules**의 포트 재작성(Port Rewrite) 기능을 활용해 서비스를 배포하는 대안적 방법을 다룹니다.

---

## 1. 아키텍처 개요

Origin Rules 방식을 사용하면 사용자의 브라우저 요청(80/443 포트)이 Cloudflare 엣지를 거칠 때, 목적지 포트가 내부 개발 포트(`3001`, `8081`, `8082`)로 변환되어 OCI VM 서버에 도달합니다.

```mermaid
graph TD
    User([유저 브라우저]) -->|HTTPS :443| CF[Cloudflare Edge]
    
    subgraph Cloudflare DNS Proxy (Origin Rules 적용)
        CF -->|Path: /v1 ➡️ :8082| API_Port[Go API Port]
        CF -->|Path: /ws ➡️ :8081| WS_Port[Go WebSocket Port]
        CF -->|기타 ➡️ :3001| FE_Port[Next.js Port]
    end
    
    subgraph OCI Compute Instance (140.245.64.172)
        API_Port --> GoAPI[Go API Server :8082]
        WS_Port --> GoWS[Go WebSocket Server :8081]
        FE_Port --> NextJS[Next.js Frontend :3001]
    end
```

---

## 2. 1단계: OCI 및 OS 방화벽 허용 설정 (필수)

이 방식은 외부(Cloudflare)가 OCI VM의 실제 IP와 커스텀 포트로 패킷을 직접 전달하므로, 해당 포트들의 통로를 열어주어야 합니다.

### A. OCI 콘솔 수신 규칙 (Ingress Rules) 설정
1. **오라클 클라우드 콘솔** 로그인 후 **Virtual Cloud Network (VCN)** ➡️ **Security Lists (보안 리스트)**로 이동합니다.
2. **수신 규칙 추가 (Add Ingress Rules)**를 클릭하고 다음 포트들을 등록합니다.
   * **소스 유형 (Source Type)**: `CIDR`
   * **소스 CIDR**: `0.0.0.0/0` *(빠른 검증용. 프로덕션 운영 시에는 Cloudflare IP 대역만 허용하는 것을 권장합니다)*
   * **IP 프로토콜**: `TCP`
   * **대상 포트 범위 (Destination Port Range)**: `3001, 8081, 8082` (쉼표로 구분)

### B. 우분투 VM 내부 방화벽 (UFW) 설정
VM 터미널에 SSH로 접속한 뒤 UFW 방화벽이 켜져있다면 해당 포트들을 허용합니다.
```bash
sudo ufw allow 3001/tcp
sudo ufw allow 8081/tcp
sudo ufw allow 8082/tcp
```

---

## 3. 2단계: Cloudflare 대시보드 설정

### A. DNS 레코드 등록 (Proxy 활성화)
1. Cloudflare 대시보드에서 도메인(예: `pl3.kr`)의 **DNS > Records** 메뉴로 이동합니다.
2. **Add record**를 클릭하여 아래 A 레코드를 추가합니다.
   * **Type**: `A`
   * **Name**: `buzz48` (즉, 완성 도메인은 `buzz48.pl3.kr`)
   * **IPv4 Address**: `140.245.64.172`
   * **Proxy status**: 🧡 **Proxied** (주황색 구름 상태 확인)
   * **TTL**: `Auto`

---

### B. Origin Rules 작성 (포트 포워딩 룰)
동일한 도메인 주소 안에서 서브 경로(`/v1`, `/ws`)에 따라 요청을 다른 내부 포트로 전달하도록 규칙을 만듭니다.

1. Cloudflare 대시보드 좌측 메뉴에서 **Rules ➡️ Origin Rules**를 클릭합니다.
2. **Create rule**을 클릭한 후 다음과 같이 채워 넣습니다.

#### 📝 규칙 1: Go REST API 서버 매핑
* **Rule name**: `Buzz48 API Port Rewrite`
* **Field**: `URI Path`
* **Operator**: `starts with`
* **Value**: `/v1`
* **Destination Port Override**: `8082`

#### 📝 규칙 2: Go WebSocket 서버 매핑
* **Rule name**: `Buzz48 WS Port Rewrite`
* **Field**: `URI Path`
* **Operator**: `starts with`
* **Value**: `/ws`
* **Destination Port Override**: `8081`

#### 📝 규칙 3: Next.js 프론트엔드 매핑 (기본 매핑)
* **Rule name**: `Buzz48 Frontend Port Rewrite`
* **Field**: `Hostname`
* **Operator**: `equals`
* **Value**: `buzz48.pl3.kr`
* *(동시에 하위 경로 조건 예외 처리)* ➡️ **And** 클릭
  * **Field**: `URI Path`
  * **Operator**: `does not start with`
  * **Value**: `/v1`
  * **And** 클릭
  * **Field**: `URI Path`
  * **Operator**: `does not start with`
  * **Value**: `/ws`
* **Destination Port Override**: `3001`

*모두 입력한 뒤 **Deploy**를 눌러 규칙을 활성화합니다.*

---

## 4. 보안 강화 팁 (Cloudflare IP만 허용하기)

포트를 `0.0.0.0/0` 전체에 공개하는 것은 외부 스캔 공격에 취약하므로, 작동이 잘 되는 것을 확인한 후에는 OCI 수신 규칙(Ingress Rules)의 **소스 CIDR**을 **Cloudflare 공식 IP 대역**으로만 제한하는 것을 권장합니다.

* Cloudflare 공식 IP 목록: [https://www.cloudflare.com/ips/](https://www.cloudflare.com/ips/)
* **IPv4 주요 범위 예시**: `173.245.48.0/20`, `103.21.244.0/22`, `103.22.200.0/22`, `103.31.4.0/22`, `141.101.64.0/18`, `108.162.192.0/18`, `190.93.240.0/20`, `188.114.96.0/20`, `197.234.240.0/22`, `198.41.128.0/17`, `162.158.0.0/15`, `104.16.0.0/13`, `104.24.0.0/14`, `172.64.0.0/13`, `131.0.72.0/22` 등
