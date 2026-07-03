package handler

import (
	"github.com/gofiber/fiber/v2"
)

// TestConsole은 브라우저에서 REST API 및 WebSocket을 즉시 테스트할 수 있는
// 모던하고 미려한 인터랙티브 웹페이지(HTML/JS)를 반환합니다.
func (d *Deps) TestConsole(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(testConsoleHTML)
}

const testConsoleHTML = `
<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>버즈48 (Buzz48) — API & WebSocket 실시간 테스트 콘솔</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;600;700&family=Outfit:wght@600;800&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0b0f19;
            --panel-bg: rgba(17, 24, 39, 0.7);
            --border-color: rgba(255, 255, 255, 0.08);
            --primary: #4f46e5;
            --primary-hover: #6366f1;
            --accent: #f43f5e;
            --text: #f3f4f6;
            --text-muted: #9ca3af;
            --green: #10b981;
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: 'Inter', sans-serif;
            background-color: var(--bg-color);
            color: var(--text);
            padding: 40px 20px;
            min-height: 100vh;
            background-image: radial-gradient(circle at 10% 20%, rgba(79, 70, 229, 0.15) 0%, transparent 40%),
                              radial-gradient(circle at 90% 80%, rgba(244, 63, 94, 0.1) 0%, transparent 40%);
            background-attachment: fixed;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        header {
            text-align: center;
            margin-bottom: 40px;
        }

        h1 {
            font-family: 'Outfit', sans-serif;
            font-size: 2.5rem;
            font-weight: 800;
            background: linear-gradient(to right, #a5b4fc, #ec4899);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 10px;
        }

        .subtitle {
            color: var(--text-muted);
            font-size: 1.1rem;
        }

        .grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 24px;
        }

        @media (max-width: 900px) {
            .grid {
                grid-template-columns: 1fr;
            }
        }

        .card {
            background: var(--panel-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 24px;
            backdrop-filter: blur(12px);
            box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.37);
            margin-bottom: 24px;
        }

        h2 {
            font-size: 1.25rem;
            font-weight: 600;
            margin-bottom: 20px;
            display: flex;
            align-items: center;
            gap: 10px;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 10px;
        }

        .badge {
            background: var(--primary);
            font-size: 0.75rem;
            padding: 2px 8px;
            border-radius: 9999px;
            text-transform: uppercase;
        }

        .form-group {
            margin-bottom: 16px;
        }

        label {
            display: block;
            font-size: 0.875rem;
            color: var(--text-muted);
            margin-bottom: 6px;
        }

        input, textarea, select {
            width: 100%;
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 10px 14px;
            color: var(--text);
            font-size: 0.95rem;
            outline: none;
            transition: all 0.2s;
        }

        input:focus, textarea:focus, select:focus {
            border-color: var(--primary);
            box-shadow: 0 0 0 2px rgba(79, 70, 229, 0.2);
        }

        .btn-group {
            display: flex;
            gap: 10px;
            flex-wrap: wrap;
        }

        button {
            background: var(--primary);
            color: white;
            border: none;
            border-radius: 8px;
            padding: 10px 20px;
            font-size: 0.95rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
        }

        button:hover {
            background: var(--primary-hover);
        }

        button.secondary {
            background: rgba(255, 255, 255, 0.1);
            color: var(--text);
        }

        button.secondary:hover {
            background: rgba(255, 255, 255, 0.15);
        }

        button.accent {
            background: var(--accent);
        }

        button.accent:hover {
            background: #e11d48;
        }

        .console {
            background: #070913;
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 16px;
            font-family: monospace;
            font-size: 0.85rem;
            height: 300px;
            overflow-y: auto;
            white-space: pre-wrap;
            color: #34d399;
            box-shadow: inset 0 2px 8px rgba(0, 0, 0, 0.8);
        }

        .console-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 8px;
        }

        .status-dot {
            width: 8px;
            height: 8px;
            background: var(--accent);
            border-radius: 50%;
            display: inline-block;
        }

        .status-dot.connected {
            background: var(--green);
            box-shadow: 0 0 8px var(--green);
        }

        .chat-container {
            display: flex;
            flex-direction: column;
            height: 350px;
            border: 1px solid var(--border-color);
            border-radius: 12px;
            background: rgba(0,0,0,0.2);
            overflow: hidden;
        }

        .chat-messages {
            flex: 1;
            padding: 16px;
            overflow-y: auto;
            display: flex;
            flex-direction: column;
            gap: 12px;
        }

        .chat-bubble {
            max-width: 80%;
            padding: 10px 14px;
            border-radius: 12px;
            font-size: 0.9rem;
            position: relative;
            line-height: 1.4;
        }

        .chat-bubble.other {
            background: rgba(255, 255, 255, 0.08);
            align-self: flex-start;
            border-top-left-radius: 0;
        }

        .chat-bubble.me {
            background: var(--primary);
            align-self: flex-end;
            border-top-right-radius: 0;
        }

        .chat-bubble .meta {
            font-size: 0.7rem;
            color: var(--text-muted);
            margin-bottom: 4px;
            font-weight: 600;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Buzz48 Interactive Console</h1>
            <p class="subtitle">REST API 및 WebSocket 실시간 통신 통합 테스트 환경</p>
        </header>

        <div class="grid">
            <div>
                <div class="card">
                    <h2><span class="badge">Session</span> 1. 익명 세션 및 프로필</h2>
                    <div class="form-group">
                        <label>현재 세션 ID (쿠키 연동)</label>
                        <input type="text" id="session-id-display" readonly placeholder="세션 발급 전">
                    </div>
                    <div class="form-group">
                        <label>현재 닉네임</label>
                        <input type="text" id="nickname-display" readonly placeholder="세션 발급 전">
                    </div>
                    <div class="btn-group">
                        <button onclick="createSession()">세션 생성 (POST)</button>
                        <button class="secondary" onclick="patchNickname()">닉네임 변경 (PATCH)</button>
                    </div>
                </div>

                <div class="card">
                    <h2><span class="badge">Post</span> 2. 게시글 작성 & 조회</h2>
                    <div class="form-group">
                        <label>게시물 제목</label>
                        <input type="text" id="post-title" value="오늘 저녁 뭐 먹을까요?">
                    </div>
                    <div class="form-group">
                        <label>본문 내용</label>
                        <textarea id="post-content" rows="2">마라탕 vs 삼겹살 투표 받습니다.</textarea>
                    </div>
                    <div class="form-group">
                        <label>카테고리</label>
                        <select id="post-category">
                            <option value="자유">자유</option>
                            <option value="경제·주식">경제·주식</option>
                            <option value="스포츠">스포츠</option>
                            <option value="연예·문화">연예·문화</option>
                        </select>
                    </div>
                    <div class="btn-group">
                        <button onclick="createPost()">게시글 작성 (POST)</button>
                        <button class="secondary" onclick="listPosts()">목록 조회 (GET)</button>
                        <button class="secondary" onclick="getHotPosts()">HOT 상위 3개 (GET)</button>
                    </div>
                </div>

                <div class="card">
                    <div class="console-header">
                        <h2>API Response Log</h2>
                        <button class="secondary" style="padding: 4px 10px; font-size: 0.8rem;" onclick="clearConsole()">Clear</button>
                    </div>
                    <div class="console" id="log-console">API 요청을 보내면 결과가 여기에 출력됩니다.</div>
                </div>
            </div>

            <div>
                <div class="card">
                    <h2>
                        <span class="badge" style="background:var(--accent)">WebSocket</span> 
                        3. 실시간 채팅 채널 
                        <span class="status-dot" id="ws-status"></span>
                    </h2>
                    
                    <div class="form-group">
                        <label>테스트할 게시물 ID (UUID)</label>
                        <input type="text" id="target-post-id" placeholder="게시물 작성 시 자동 입력됩니다">
                    </div>

                    <div class="btn-group" style="margin-bottom: 20px;">
                        <button class="accent" id="ws-conn-btn" onclick="toggleWS()">WebSocket 연결</button>
                        <button class="secondary" onclick="sendSync()">과거메시지 복구 (sync)</button>
                    </div>

                    <div class="chat-container">
                        <div class="chat-messages" id="chat-box">
                            <div class="chat-bubble other">
                                <div class="meta">시스템</div>
                                <div>게시물 ID를 지정하고 WebSocket을 연결하면 실시간 채팅이 활성화됩니다.</div>
                            </div>
                        </div>
                        <div class="chat-input-zone">
                            <input type="text" id="chat-msg-input" placeholder="메시지를 입력하세요 (최대 500자)" onkeydown="if(event.key==='Enter') sendWSMessage()">
                            <button onclick="sendWSMessage()">전송</button>
                        </div>
                    </div>
                </div>

                <div class="card" id="post-meta-card" style="display:none;">
                    <h2>게시물 메타 정보 (실시간 수신)</h2>
                    <div style="display:grid; grid-template-columns: 1fr 1fr; gap:10px; font-size:0.9rem;">
                        <div>상태: <span id="meta-state" style="font-weight:600; color:var(--green)">-</span></div>
                        <div>남은 수명: <span id="meta-timer" style="font-weight:600; color:var(--accent)">-</span></div>
                        <div>동접자 수: <span id="meta-conn" style="font-weight:600;">-</span>명</div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script>
        const API_URL = window.location.origin + "/v1";
        const WS_URL = "ws://" + window.location.hostname + ":8083/v1";
        
        let wsConn = null;
        let activeSessionID = "";
        let activeNickname = "";
        let currentPostID = "";

        function log(title, obj) {
            const consoleEl = document.getElementById("log-console");
            const time = new Date().toLocaleTimeString();
            consoleEl.innerHTML = "[" + time + "] " + title + "\n" + JSON.stringify(obj, null, 2) + "\n\n" + consoleEl.innerHTML;
        }

        function clearConsole() {
            document.getElementById("log-console").innerHTML = "";
        }

        async function createSession() {
            try {
                const res = await fetch(API_URL + "/sessions", { method: "POST" });
                const data = await res.json();
                activeSessionID = data.session_id;
                activeNickname = data.nickname;
                
                document.getElementById("session-id-display").value = activeSessionID;
                document.getElementById("nickname-display").value = activeNickname;
                log("POST /sessions Success", data);
            } catch (err) {
                log("POST /sessions Error", err.message);
            }
        }

        async function patchNickname() {
            const newNick = prompt("변경할 닉네임을 입력하세요:");
            if (!newNick) return;

            try {
                const res = await fetch(API_URL + "/sessions/me/nickname", {
                    method: "PATCH",
                    headers: { 
                        "Content-Type": "application/json",
                        "Authorization": "Bearer " + activeSessionID
                    },
                    body: JSON.stringify({ nickname: newNick })
                });
                const data = await res.json();
                if (res.ok) {
                    activeNickname = data.nickname;
                    document.getElementById("nickname-display").value = activeNickname;
                }
                log("PATCH /sessions/me/nickname Result", data);
            } catch (err) {
                log("PATCH /sessions/me/nickname Error", err.message);
            }
        }

        async function createPost() {
            const title = document.getElementById("post-title").value;
            const content = document.getElementById("post-content").value;
            const category = document.getElementById("post-category").value;

            try {
                const res = await fetch(API_URL + "/posts", {
                    method: "POST",
                    headers: {
                        "Content-Type": "application/json",
                        "Authorization": "Bearer " + activeSessionID
                    },
                    body: JSON.stringify({
                        title: title,
                        content: content,
                        category: category,
                        image_urls: [],
                        idempotency_key: "idemp-" + Math.random()
                    })
                });
                const data = await res.json();
                if (res.ok) {
                    currentPostID = data.post_id;
                    document.getElementById("target-post-id").value = currentPostID;
                    document.getElementById("post-meta-card").style.display = "block";
                }
                log("POST /posts Success", data);
            } catch (err) {
                log("POST /posts Error", err.message);
            }
        }

        async function listPosts() {
            try {
                const res = await fetch(API_URL + "/posts");
                const data = await res.json();
                log("GET /posts Success", data);
            } catch (err) {
                log("GET /posts Error", err.message);
            }
        }

        async function getHotPosts() {
            try {
                const res = await fetch(API_URL + "/posts/hot");
                const data = await res.json();
                log("GET /posts/hot Success", data);
            } catch (err) {
                log("GET /posts/hot Error", err.message);
            }
        }

        function toggleWS() {
            const postID = document.getElementById("target-post-id").value;
            if (!postID) {
                alert("대상 게시물 ID를 입력하세요.");
                return;
            }
            if (!activeSessionID) {
                alert("먼저 세션을 생성해 주세요.");
                return;
            }

            if (wsConn) {
                wsConn.close();
                return;
            }

            const url = WS_URL + "/ws/posts/" + postID + "?token=" + activeSessionID;
            wsConn = new WebSocket(url);

            const statusDot = document.getElementById("ws-status");
            const connBtn = document.getElementById("ws-conn-btn");

            wsConn.onopen = () => {
                statusDot.classList.add("connected");
                connBtn.innerText = "WebSocket 해제";
                appendChatMessage("System", "WebSocket 연결이 성립되었습니다.", false);
            };

            wsConn.onclose = () => {
                statusDot.classList.remove("connected");
                connBtn.innerText = "WebSocket 연결";
                wsConn = null;
                appendChatMessage("System", "WebSocket 연결이 닫혔습니다.", false);
            };

            wsConn.onmessage = (event) => {
                const data = JSON.parse(event.data);
                handleWSEvent(data);
            };
        }

        function handleWSEvent(evt) {
            console.log("WS Recv: ", evt);
            const chatBox = document.getElementById("chat-box");
            
            switch (evt.type) {
                case "post_snapshot":
                    const payload = evt.payload;
                    document.getElementById("meta-state").innerText = payload.state;
                    document.getElementById("meta-conn").innerText = payload.conn_count;
                    document.getElementById("meta-timer").innerText = payload.remaining_seconds + "초";
                    
                    chatBox.innerHTML = "";
                    payload.recent_messages.forEach(msg => {
                        appendChatMessage(msg.sender_nickname, msg.content, msg.sender_nickname === activeNickname);
                    });
                    break;
                case "message_received":
                    appendChatMessage(evt.payload.sender_nickname, evt.payload.content, evt.payload.sender_nickname === activeNickname);
                    break;
                case "conn_count_updated":
                    document.getElementById("meta-conn").innerText = evt.payload.conn_count;
                    break;
                case "post_state_changed":
                    document.getElementById("meta-state").innerText = evt.payload.state;
                    appendChatMessage("System", "게시판 상태가 " + evt.payload.state + "로 변경되었습니다.", false);
                    break;
                case "reaction_updated":
                    appendChatMessage("System", "[반응] 메시지 " + evt.payload.message_id.substring(0,6) + "... 에 이모지 " + evt.payload.emoji + "가 업데이트되었습니다. (" + evt.payload.count + "개)", false);
                    break;
                case "sync_result":
                    evt.payload.missed_messages.forEach(msg => {
                        appendChatMessage(msg.sender_nickname, msg.content + " [동기화됨]", msg.sender_nickname === activeNickname);
                    });
                    break;
                case "error":
                    appendChatMessage("Error", evt.payload.message, false);
                    break;
            }
        }

        function appendChatMessage(sender, text, isMe) {
            const chatBox = document.getElementById("chat-box");
            const div = document.createElement("div");
            div.className = "chat-bubble " + (isMe ? "me" : "other");
            div.innerHTML = "<div class=\"meta\">" + sender + "</div><div>" + text + "</div>";
            chatBox.appendChild(div);
            chatBox.scrollTop = chatBox.scrollHeight;
        }

        function sendWSMessage() {
            const input = document.getElementById("chat-msg-input");
            const val = input.value.trim();
            if (!val || !wsConn) return;

            const sendObj = {
                event: "send_message",
                payload: {
                    content: val,
                    reply_to: null
                }
            };
            wsConn.send(JSON.stringify(sendObj));
            input.value = "";
        }

        function sendSync() {
            if (!wsConn) return;
            const lastKnown = prompt("마지막으로 수신했던 메시지 ID(UUID)를 입력하세요:");
            if (!lastKnown) return;

            const syncObj = {
                event: "sync",
                payload: {
                    last_message_id: lastKnown
                }
            };
            wsConn.send(JSON.stringify(syncObj));
        }
    </script>
</body>
</html>
`;
