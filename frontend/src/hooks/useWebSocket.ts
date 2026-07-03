'use client';

import { useEffect, useRef, useState, useCallback } from 'react';

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

		const wsUrl = `ws://localhost:8083/v1/ws/posts/${postID}?token=${sessionID}`;
		const socket = new WebSocket(wsUrl);
		ws.current = socket;

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
				const data = JSON.parse(event.data);
				
				switch (data.type) {
					case 'post_snapshot':
						const payload = data.payload;
						setPostState(payload.state);
						setConnCount(payload.conn_count);
						if (payload.recent_messages && messages.length === 0) {
							setMessages(payload.recent_messages);
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
						// 누락된 메시지 복구 적용
						const missed = data.payload.missed_messages || [];
						if (missed.length > 0) {
							console.log(`📥 [WebSocket] 동기화로 ${missed.length}개 누락 메시지 복원`);
							setMessages(prev => [...prev, ...missed]);
						}
						break;
				}
			} catch (err) {
				console.error('웹소켓 데이터 파싱 오류:', err);
			}
		};

		socket.onclose = () => {
			console.log('🔌 [WebSocket] 연결 종료');
			
			// 24시간 LIVE 수명이 살아있는 상태에서 의도치 않게 닫힌 경우, 최대 5번 자동 재접속 시도
			if (postStateRef.current === 'LIVE' && reconnectCount.current < 5) {
				const delay = Math.min(1000 * Math.pow(2, reconnectCount.current), 10000); // 지수 백오프
				reconnectCount.current += 1;
				console.warn(`🔌 [WebSocket] ${delay/1000}초 후 자동 재접속을 시도합니다. (시도: ${reconnectCount.current}/5)`);
				setTimeout(() => {
					connect();
				}, delay);
			}
		};

		socket.onerror = (err) => {
			console.error('🔌 [WebSocket] 에러 발생:', err);
			socket.close();
		};
	}, [postID, sessionID]);

	useEffect(() => {
		connect();
		return () => {
			if (ws.current) {
				ws.current.close();
			}
		};
	}, [connect]);

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
	}, [postState]);

	// 반응 이모지 토글 트리거
	const sendReaction = useCallback((emoji: string) => {
		if (!ws.current || ws.current.readyState !== WebSocket.OPEN || postState === 'READ') return;

		ws.current.send(JSON.stringify({
			type: 'toggle_reaction',
			payload: { emoji }
		}));
	}, [postState]);

	return {
		messages,
		connCount,
		postState,
		reactions,
		sendMessage,
		sendReaction
	};
}
