'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import { getPostMessagesAPI } from '../lib/api';

interface Message {
	message_id: string;
	sender_nickname: string;
	content: string;
	parent_id?: string;
	parent_sender_nickname?: string;
	created_at: string;
}

interface UseWebSocketProps {
	postID: string;
	sessionID: string;
	initialState: 'LIVE' | 'READ';
}

export function useWebSocket({ postID, sessionID, initialState }: UseWebSocketProps) {
	const [messages, setMessages] = useState<Message[]>([]);
	const [connCount, setConnCount] = useState(1);
	const [postState, setPostState] = useState<'LIVE' | 'READ'>(initialState);
	const [reactions, setReactions] = useState<Record<string, number>>({
		'👍': 0,
		'❤️': 0,
		'😂': 0,
		'😡': 0
	});

	const ws = useRef<WebSocket | null>(null);
	const reconnectCount = useRef(0);
	const lastMessageIdRef = useRef<string | null>(null);
	const postStateRef = useRef<'LIVE' | 'READ'>(initialState);

	const connectRef = useRef<() => void>(() => {});
	const isClosedIntentionallyRef = useRef(false);

	// 최신 postState 참조 보존
	useEffect(() => {
		postStateRef.current = postState;
	}, [postState]);


	// 메시지 배열 갱신 시 마지막 메시지 ID 참조 보존 (싱크 동기화의 기준점)
	useEffect(() => {
		if (messages.length > 0) {
			lastMessageIdRef.current = messages[messages.length - 1].message_id;
		}
	}, [messages]);

	const connect = useCallback(() => {
		if (postStateRef.current === 'READ') return; // 만료 글은 소켓 연결 불필요
		if (!sessionID) {
			console.log('🔌 [WebSocket] 세션 ID가 아직 준비되지 않아 연결을 보류합니다.');
			return;
		}

		const wsBase = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:8083/v1/ws";
		const wsUrl = `${wsBase}/posts/${postID}?token=${sessionID}`;
		const socket = new WebSocket(wsUrl);
		ws.current = socket;
		isClosedIntentionallyRef.current = false; // 신규 연결 수립 시 플래그 리셋

		socket.onopen = () => {
			console.log('🔌 [WebSocket] 연결 성공:', postID);
			reconnectCount.current = 0; // 재연결 횟수 초기화

			// 재연결된 경우, 이전 마지막 메시지 ID를 기반으로 sync 동기화 요청 (PRD §3.1)
			if (lastMessageIdRef.current) {
				console.log('🔄 [WebSocket] 재연결 동기화 요청: last_message_id =', lastMessageIdRef.current);
				socket.send(JSON.stringify({
					type: 'sync',
					payload: {
						last_message_id: lastMessageIdRef.current
					}
				}));
			}
		};

		socket.onmessage = (event) => {
			try {
				// 개행 (\n) 문자로 구분된 다중 패킷 파싱 지원 (WritePump 일괄 Flush 호환)
				const lines = event.data.split('\n');
				for (const line of lines) {
					if (!line.trim()) continue;
					const data = JSON.parse(line);
					
					switch (data.type) {
						case 'post_snapshot':
							const payload = data.payload;
							setPostState(payload.state);
							setConnCount(payload.conn_count);
							if (payload.recent_messages && messages.length === 0) {
								const normalized = payload.recent_messages.map((m: any) => ({
									message_id: m.id || m.message_id,
									sender_nickname: m.sender_nickname,
									content: m.content,
									parent_id: m.reply_to_id || m.parent_id,
									parent_sender_nickname: m.parent_sender_nickname || '',
									created_at: m.created_at
								}));
								setMessages(normalized);
							}
							if (payload.reactions) {
								const rxMap: Record<string, number> = { '👍': 0, '❤️': 0, '😂': 0, '😡': 0 };
								payload.reactions.forEach((rx: any) => {
									rxMap[rx.emoji] = rx.count;
								});
								setReactions(rxMap);
							}
							break;

						case 'message_received':
							setMessages(prev => [...prev, data.payload]);
							break;

						case 'conn_count_updated':
							setConnCount(data.payload.conn_count);
							break;

						case 'post_state_changed':
							setPostState(data.payload.state);
							break;

						case 'reaction_updated':
							const updatedRx = data.payload;
							setReactions(prev => ({
								...prev,
								[updatedRx.emoji]: updatedRx.count
							}));
							break;

						case 'sync_result':
							const missed = data.payload.missed_messages || [];
							if (missed.length > 0) {
								console.log(`📥 [WebSocket] 동기화로 ${missed.length}개 누락 메시지 복원`);
								setMessages(prev => [...prev, ...missed]);
							}
							break;
					}
				}
			} catch (err) {
				console.error('웹소켓 데이터 파싱 오류:', err);
			}
		};

		socket.onclose = () => {
			console.log('🔌 [WebSocket] 연결 종료');
			
			// 언마운트 등 의도적으로 소켓을 닫은 경우 재연결을 원천 봉쇄 (고스트 커넥션 방지)
			if (isClosedIntentionallyRef.current) {
				console.log('🔌 [WebSocket] 의도된 연결 종료이므로 재접속을 시도하지 않습니다.');
				return;
			}

			// 24시간 LIVE 수명이 살아있는 상태에서 의도치 않게 닫힌 경우, 최대 5번 자동 재접속 시도
			if (postStateRef.current === 'LIVE' && reconnectCount.current < 5) {
				const delay = Math.min(1000 * Math.pow(2, reconnectCount.current), 10000); // 지수 백오프
				reconnectCount.current += 1;
				console.warn(`🔌 [WebSocket] ${delay/1000}초 후 자동 재접속을 시도합니다. (시도: ${reconnectCount.current}/5)`);
				setTimeout(() => {
					connectRef.current();
				}, delay);
			}
		};

		socket.onerror = (err) => {
			console.error('🔌 [WebSocket] 에러 발생:', err);
			socket.close();
		};
	}, [postID, sessionID]);

	// 최신 connect 참조 보존
	useEffect(() => {
		connectRef.current = connect;
	}, [connect]);

	useEffect(() => {
		connect();
		return () => {
			if (ws.current) {
				isClosedIntentionallyRef.current = true; // 언마운트 시 명시적 종료 플래그 활성화
				ws.current.close();
			}
		};
	}, [connect]);

	// READ 상태일 때 과거 메시지 목록 아카이브 HTTP 조회 연동
	useEffect(() => {
		if (postState === 'READ') {
			const fetchArchive = async () => {
				try {
					const data = await getPostMessagesAPI(postID);
					const normalized = data.messages.map((m: any) => ({
						message_id: m.message_id,
						sender_nickname: m.sender_nickname,
						content: m.content,
						parent_id: m.reply_to,
						parent_sender_nickname: '',
						created_at: m.created_at
					}));
					// 오래된 메시지가 위로 오게 배열 정렬
					setMessages(normalized.reverse());
				} catch (err) {
					console.error('과거 대화 아카이브 조회 실패:', err);
				}
			};
			fetchArchive();
		}
	}, [postID, postState]);

	// 메시지 전송 트리거
	const sendMessage = useCallback((content: string, parentID?: string) => {
		if (!ws.current || ws.current.readyState !== WebSocket.OPEN || postState === 'READ') return;

		const payload: any = {
			type: 'send_message',
			payload: { content }
		};

		if (parentID) {
			payload.payload.parent_id = parentID;
		}

		ws.current.send(JSON.stringify(payload));
	}, [postState, sessionID]);

	// 반응 이모지 토글 트리거
	const sendReaction = useCallback((emoji: string) => {
		if (!ws.current || ws.current.readyState !== WebSocket.OPEN || postState === 'READ') return;

		ws.current.send(JSON.stringify({
			type: 'toggle_reaction',
			payload: {
				message_id: postID,
				emoji
			}
		}));
	}, [postState, sessionID, postID]);

	return {
		messages,
		connCount,
		postState,
		reactions,
		sendMessage,
		sendReaction
	};
}
