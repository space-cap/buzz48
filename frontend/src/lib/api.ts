const API_BASE = "http://localhost:8082/v1";

export interface SessionInfo {
	session_id: string;
	nickname: string;
	created_at: string;
}

export interface PostItem {
	post_id: string;
	title: string;
	category: string;
	state: "LIVE" | "READ";
	conn_count: number;
	remaining_seconds: number;
	is_new: boolean;
	is_premium: boolean;
	created_at?: string;
}

export interface HotPostItem {
	post_id: string;
	title: string;
	category: string;
	conn_count: number;
	score: number;
	remaining_seconds: number;
}

// Authorization 헤더 구성을 위한 세션 획득 헬퍼
function getSessionHeader(): Record<string, string> {
	if (typeof window === "undefined") return {};
	const sessionID = localStorage.getItem("session_id");
	return sessionID ? { "Authorization": `Bearer ${sessionID}` } : {};
}

// 1. 익명 세션 생성
export async function createSessionAPI(): Promise<SessionInfo> {
	const res = await fetch(`${API_BASE}/sessions`, { method: "POST" });
	if (!res.ok) {
		throw new Error("세션 생성 실패");
	}
	const data = await res.json();
	localStorage.setItem("session_id", data.session_id);
	localStorage.setItem("nickname", data.nickname);
	return data;
}

// 2. 닉네임 변경
export async function patchNicknameAPI(nickname: string): Promise<{ nickname: string }> {
	const res = await fetch(`${API_BASE}/sessions/me/nickname`, {
		method: "PATCH",
		headers: {
			"Content-Type": "application/json",
			...getSessionHeader(),
		},
		body: JSON.stringify({ nickname }),
	});
	if (!res.ok) {
		const err = await res.json();
		throw new Error(err.error?.message || "닉네임 변경 실패");
	}
	const data = await res.json();
	localStorage.setItem("nickname", data.nickname);
	return data;
}

// 3. 게시글 작성
export async function createPostAPI(post: {
	title: string;
	content: string;
	category: string;
	image_urls: string[];
	idempotency_key?: string;
}): Promise<PostItem> {
	const res = await fetch(`${API_BASE}/posts`, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
			...getSessionHeader(),
		},
		body: JSON.stringify({
			...post,
			idempotency_key: post.idempotency_key || `idemp-${Math.random()}`,
		}),
	});
	if (!res.ok) {
		const err = await res.json();
		throw new Error(err.error?.message || "게시물 작성 실패");
	}
	return res.json();
}

// 4. 최신 게시글 목록 조회
export async function getPostsAPI(params: {
	category?: string;
	cursor?: string;
	limit?: number;
	search?: string;
}): Promise<{ posts: PostItem[]; next_cursor: string | null }> {
	const query = new URLSearchParams();
	if (params.category) query.append("category", params.category);
	if (params.cursor) query.append("cursor", params.cursor);
	if (params.limit) query.append("limit", params.limit.toString());
	if (params.search) query.append("search", params.search);

	const res = await fetch(`${API_BASE}/posts?${query.toString()}`);
	if (!res.ok) {
		throw new Error("게시물 조회 실패");
	}
	return res.json();
}

// 5. 실시간 HOT 3 조회
export async function getHotPostsAPI(): Promise<{ posts: HotPostItem[] }> {
	const res = await fetch(`${API_BASE}/posts/hot`);
	if (!res.ok) {
		throw new Error("HOT 게시물 조회 실패");
	}
	return res.json();
}

// 6. 게시글 상세 조회
export async function getPostDetailAPI(postID: string): Promise<any> {
	const res = await fetch(`${API_BASE}/posts/${postID}`);
	if (!res.ok) {
		const err = await res.json();
		throw new Error(err.error?.message || "게시물 상세 조회 실패");
	}
	return res.json();
}

// 7. 내 세션 정보 조회
export async function getMySessionAPI(): Promise<SessionInfo> {
	const res = await fetch(`${API_BASE}/sessions/me`, {
		method: "GET",
		headers: getSessionHeader(),
	});
	if (!res.ok) {
		throw new Error("세션 정보 조회 실패");
	}
	return res.json();
}

// 8. READ 상태 게시글의 과거 아카이브 메시지 조회
export async function getPostMessagesAPI(
	postID: string,
	cursor?: string,
	limit?: number
): Promise<{ messages: any[]; next_cursor: string | null }> {
	const query = new URLSearchParams();
	if (cursor) query.append("cursor", cursor);
	if (limit) query.append("limit", limit.toString());

	const res = await fetch(`${API_BASE}/posts/${postID}/messages?${query.toString()}`);
	if (!res.ok) {
		throw new Error("과거 대화 조회 실패");
	}
	return res.json();
}
