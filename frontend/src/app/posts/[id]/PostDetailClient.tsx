'use client';

import React, { useEffect, useState, useRef } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { getPostDetailAPI } from '../../../lib/api';
import TimerText from '../../../components/TimerText';
import { useWebSocket } from '../../../hooks/useWebSocket';

// ────────────────────────────────────────────────────────
// 미니멀 SVG 아이콘 컴포넌트 (AI 이모지 대체)
// ────────────────────────────────────────────────────────
const FlameIcon = ({ size = 14, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', ...style }}>
		<path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z"></path>
	</svg>
);

const EyeIcon = ({ size = 14, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', ...style }}>
		<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
		<circle cx="12" cy="12" r="3"></circle>
	</svg>
);

const UserIcon = ({ size = 14, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', ...style }}>
		<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
		<circle cx="12" cy="7" r="4"></circle>
	</svg>
);

const MessageIcon = ({ size = 14, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', ...style }}>
		<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
	</svg>
);

const BookIcon = ({ size = 14, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', ...style }}>
		<path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path>
		<path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path>
	</svg>
);

const ReplyIcon = ({ size = 12, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', marginRight: '4px', ...style }}>
		<polyline points="9 10 4 15 9 20"></polyline>
		<path d="M20 4v7a4 4 0 0 1-4 4H4"></path>
	</svg>
);

interface Message {
	message_id: string;
	sender_nickname: string;
	content: string;
	parent_id?: string;
	parent_sender_nickname?: string;
	created_at: string;
}

interface PostDetailClientProps {
	postID: string;
}

export default function PostDetailClient({ postID }: PostDetailClientProps) {
	const router = useRouter();

	// 상태 제어
	const [post, setPost] = useState<any>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState('');
	const [isLeftCollapsed, setIsLeftCollapsed] = useState(false);
	const [mobileTab, setMobileTab] = useState<'post' | 'chat'>('post');
	const [isMobile, setIsMobile] = useState(false);

	const [inputMsg, setInputMsg] = useState('');
	const [replyTarget, setReplyTarget] = useState<Message | null>(null);

	const chatEndRef = useRef<HTMLDivElement | null>(null);
	const [sessionID, setSessionID] = useState('');

	// 0.5 세션 ID 동적 감시 및 마운트 로드
	useEffect(() => {
		setSessionID(localStorage.getItem('session_id') || '');

		const handleSessionSync = () => {
			setSessionID(localStorage.getItem('session_id') || '');
		};
		window.addEventListener('storage', handleSessionSync);
		window.addEventListener('nicknameChanged', handleSessionSync);

		return () => {
			window.removeEventListener('storage', handleSessionSync);
			window.removeEventListener('nicknameChanged', handleSessionSync);
		};
	}, []);

	// 1. 게시글 데이터 API 로드 (최초 마운트)
	useEffect(() => {
		const loadDetail = async () => {
			try {
				const data = await getPostDetailAPI(postID);
				setPost(data);
				setLoading(false);
			} catch (err: any) {
				setError(err.message || '게시글 상세 조회에 실패했습니다.');
				setLoading(false);
			}
		};
		loadDetail();
	}, [postID]);

	// 0. 화면 너비 감지 리사이즈 리스너
	useEffect(() => {
		const handleResize = () => {
			setIsMobile(window.innerWidth <= 768);
		};
		handleResize();
		window.addEventListener('resize', handleResize);
		return () => window.removeEventListener('resize', handleResize);
	}, []);

	// 2. 고도화된 웹소켓 커스텀 훅 바인딩 (자동 재연결 및 sync 내장)
	const {
		messages,
		connCount,
		postState,
		reactions,
		sendMessage,
		sendReaction
	} = useWebSocket({
		postID,
		sessionID,
		initialState: post?.state || 'LIVE'
	});

	// 최초 메시지 로드 및 로딩 해제 시 즉시 최하단으로 이동 (스냅)
	const prevLengthRef = useRef(0);
	useEffect(() => {
		if (loading || messages.length === 0) return;

		const scrollToBottom = (behavior: ScrollBehavior) => {
			// rAF 2중 래핑 — React 렌더 → 브라우저 페인트 완료 후 스크롤 실행 보장
			requestAnimationFrame(() => {
				requestAnimationFrame(() => {
					chatEndRef.current?.scrollIntoView({ behavior });
				});
			});
		};

		if (prevLengthRef.current === 0) {
			// 초기 복원 시 → 즉시 최하단 스크롤
			scrollToBottom('instant' as ScrollBehavior);
		} else {
			// 신규 메시지 수신 시 → 부드럽게 스크롤
			scrollToBottom('smooth');
		}
		prevLengthRef.current = messages.length;
	}, [messages, loading]);

	// 메시지 전송 핸들러
	const handleSendMessageSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		if (!inputMsg.trim() || postState === 'READ') return;

		sendMessage(inputMsg.trim(), replyTarget?.message_id);
		setInputMsg('');
		setReplyTarget(null);
	};

	if (loading) return <div style={styles.loading}>정보를 불러오는 중입니다...</div>;
	if (error || !post) {
		return (
			<div style={styles.errorContainer}>
				<p>⚠️ {error || '게시글이 존재하지 않거나 만료(48시간 경과)되었습니다.'}</p>
				<button onClick={() => router.push('/')} style={styles.backBtn}>목록으로 복귀</button>
			</div>
		);
	}

	return (
		<div style={styles.container}>
			{/* 모바일 상단 탭 단추 바 */}
			<div style={{
				...styles.mobileTabBar,
				display: isMobile ? 'flex' : 'none'
			}}>
				<button 
					onClick={() => setMobileTab('post')} 
					style={{ ...styles.mobileTabBtn, borderBottom: mobileTab === 'post' ? '3px solid var(--primary)' : 'none' }}
				>
					<BookIcon style={{ marginRight: '6px' }} />
					원글 보기
				</button>
				<button 
					onClick={() => setMobileTab('chat')} 
					style={{ ...styles.mobileTabBtn, borderBottom: mobileTab === 'chat' ? '3px solid var(--primary)' : 'none' }}
				>
					<MessageIcon style={{ marginRight: '6px' }} />
					실시간 댓글 (<EyeIcon size={12} style={{ marginRight: '2px' }} /> {connCount}명)
				</button>
			</div>

			<div style={{
				...styles.splitWrapper,
				flexDirection: isMobile ? 'column' : 'row'
			}}>
				{/* ────────────────────────────────────────────────────────
				   좌측 패널 (40%) - 원문 영역 (접기/토글 지원)
				   ──────────────────────────────────────────────────────── */}
				<div style={{
					...styles.leftPanel,
					width: isMobile ? '100%' : (isLeftCollapsed ? '0px' : '40%'),
					opacity: isMobile ? 1 : (isLeftCollapsed ? 0 : 1),
					padding: isMobile ? '20px' : (isLeftCollapsed ? '0px' : '24px'),
					display: !isMobile ? 'flex' : (mobileTab === 'post' ? 'flex' : 'none'),
					borderRight: isMobile ? 'none' : '1px solid var(--border-color)',
					borderBottom: isMobile && mobileTab === 'post' ? '1px solid var(--border-color)' : 'none',
				}}>
					{/* 접기 단추 */}
					{!isMobile && (
						<button 
							onClick={() => setIsLeftCollapsed(true)} 
							style={styles.collapseBtn}
							title="채팅 넓게 보기"
						>
							◀
						</button>
					)}

					<div style={styles.leftContent}>
						<Link href="/" style={styles.backLink}>
							← 목록으로 돌아가기
						</Link>
						<div style={styles.postMeta}>
							<span style={styles.categoryBadge}>{post.category}</span>
							<TimerText initialSeconds={post.remaining_seconds} state={postState} />
						</div>

						<h1 style={styles.postTitle}>{post.title}</h1>
						<div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '0.85rem', color: 'var(--text-muted)', marginBottom: '16px' }}>
							<span>작성자: <UserIcon style={{ marginRight: '4px' }} /> {post.creator_nickname || '익명 세션 유저'}</span>
							<span><FlameIcon style={{ marginRight: '4px' }} /> 조회 {post.view_count ?? 0}</span>
						</div>

						<hr style={styles.divider} />

						<div style={styles.postBody}>
							{post.content || '본문 내용이 없는 게시물입니다.'}
						</div>
					</div>
				</div>

				{/* 좌측 패널이 접혔을 때의 펴기 단추 */}
				{!isMobile && isLeftCollapsed && (
					<button 
						onClick={() => setIsLeftCollapsed(false)} 
						style={styles.expandBtn}
						title="원글 보기"
					>
						▶
					</button>
				)}

				{/* ────────────────────────────────────────────────────────
				   우측 패널 (60%) - 실시간 채팅 & 반응 이모지 영역
				   ──────────────────────────────────────────────────────── */}
				<div style={{
					...styles.rightPanel,
					width: isMobile ? '100%' : (isLeftCollapsed ? '100%' : '60%'),
					display: !isMobile ? 'flex' : (mobileTab === 'chat' ? 'flex' : 'none'),
				}}>
					{/* 채팅 상단 메타 바 */}
					<div style={styles.chatHeader}>
						<span style={styles.chatTitle}>
							<MessageIcon style={{ marginRight: '6px' }} />
							실시간 댓글
						</span>
						<div style={styles.chatMetaZone}>
							<span style={styles.activeUsers}>
								<EyeIcon size={12} style={{ marginRight: '4px' }} />
								{connCount}명 참여 중
							</span>
							<span style={{ 
								...styles.stateBadge,
								backgroundColor: postState === 'LIVE' ? 'rgba(16, 185, 129, 0.1)' : 'rgba(245, 158, 11, 0.1)',
								color: postState === 'LIVE' ? 'var(--green)' : 'var(--text-muted)'
							}}>
								{postState === 'LIVE' ? 'LIVE' : 'READ ONLY'}
							</span>
						</div>
					</div>

					{/* 채팅 메시지 리스트 타임라인 */}
					<div style={styles.chatTimeline}>
						{messages.length === 0 ? (
							<div style={styles.emptyChat}>
								아직 작성된 댓글이 없습니다. 첫 댓글을 남겨보세요!
							</div>
						) : (
							messages.map((msg) => (
								<div key={msg.message_id} style={styles.chatBubbleRow}>
									{msg.parent_id && (
										<div style={styles.replyContext}>
											<ReplyIcon style={{ color: 'var(--primary)' }} />
											{msg.parent_sender_nickname} 님의 말에 답글:
										</div>
									)}
									
									<div style={styles.bubbleMain}>
										<span style={styles.senderName}>{msg.sender_nickname}</span>
										<div 
											style={styles.bubbleText}
											onClick={() => {
												if (postState === 'LIVE') setReplyTarget(msg);
											}}
											title="이 메시지에 답장하기"
										>
											{msg.content}
										</div>
									</div>
								</div>
							))
						)}
						<div ref={chatEndRef} />
					</div>

					{/* 4종 반응 이모지 버튼 단 */}
					<div style={styles.reactionBar}>
						{Object.keys(reactions).map((emoji) => (
							<button
								key={emoji}
								onClick={() => sendReaction(emoji)}
								style={{
									...styles.reactBtn,
									opacity: postState === 'READ' ? 0.6 : 1,
									cursor: postState === 'READ' ? 'default' : 'pointer'
								}}
								disabled={postState === 'READ'}
							>
								<span style={styles.reactEmoji}>{emoji}</span>
								<span style={styles.reactCount}>{reactions[emoji]}</span>
							</button>
						))}
					</div>

					{/* 답글 대상 캔슬 배너 */}
					{replyTarget && (
						<div style={styles.replyBanner}>
							<span>
								<ReplyIcon style={{ color: 'var(--primary)' }} />
								<b>{replyTarget.sender_nickname}</b> 님에게 답글 작성 중...
							</span>
							<button onClick={() => setReplyTarget(null)} style={styles.replyCancelBtn}>&times;</button>
						</div>
					)}

					{/* 댓글 입력 영역 */}
					{postState === 'LIVE' ? (
						<form onSubmit={handleSendMessageSubmit} style={styles.chatForm}>
							<input
								type="text"
								value={inputMsg}
								onChange={e => setInputMsg(e.target.value)}
								placeholder="댓글을 입력하세요..."
								style={styles.chatInput}
								maxLength={500}
							/>
							<button type="submit" style={styles.sendBtn}>등록</button>
						</form>
					) : (
						<div style={styles.lockedBanner}>
							🔒 이 게시물은 24시간이 경과하여 읽기 전용(READ) 모드입니다. 댓글 작성이 비활성화되었습니다.
						</div>
					)}
				</div>
			</div>
		</div>
	);
}

const styles = {
	loading: {
		padding: '80px 20px',
		textAlign: 'center' as const,
		fontSize: '1.1rem',
		color: 'var(--text-muted)',
	},
	errorContainer: {
		padding: '80px 20px',
		textAlign: 'center' as const,
		maxWidth: '500px',
		margin: '0 auto',
	},
	backBtn: {
		background: 'var(--primary)',
		border: 'none',
		borderRadius: '8px',
		color: 'white',
		padding: '10px 20px',
		marginTop: '20px',
		cursor: 'pointer',
		fontWeight: 600,
	},
	container: {
		width: '100%',
		height: 'calc(100vh - 120px)',
		display: 'flex',
		flexDirection: 'column' as const,
	},
	mobileTabBar: {
		display: 'none',
		background: 'var(--panel-bg)',
		borderBottom: '1px solid var(--border-color)',
		height: '48px',
	},
	mobileTabBtn: {
		flex: 1,
		background: 'none',
		border: 'none',
		fontSize: '0.9rem',
		fontWeight: 700,
		color: 'var(--text-main)',
		cursor: 'pointer',
	},
	splitWrapper: {
		display: 'flex',
		flex: 1,
		overflow: 'hidden',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '16px',
		boxShadow: 'var(--shadow-md)',
	},
	leftPanel: {
		display: 'flex',
		flexDirection: 'column' as const,
		borderRight: '1px solid var(--border-color)',
		position: 'relative' as const,
		transition: 'width var(--transition-speed) ease, padding var(--transition-speed) ease, opacity var(--transition-speed) ease',
		overflowY: 'auto' as const,
		flexShrink: 0,
		width: '40%',
	},
	collapseBtn: {
		position: 'absolute' as const,
		top: '16px',
		right: '16px',
		width: '32px',
		height: '32px',
		borderRadius: '50%',
		border: '1px solid var(--border-color)',
		background: 'var(--bg-color)',
		color: 'var(--text-muted)',
		cursor: 'pointer',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		fontSize: '0.8rem',
		zIndex: 5,
	},
	expandBtn: {
		width: '48px',
		borderRight: '1px solid var(--border-color)',
		background: 'var(--panel-bg)',
		color: 'var(--text-muted)',
		cursor: 'pointer',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		fontSize: '1rem',
		fontWeight: 700,
		transition: 'background 0.2s',
	},
	backLink: {
		display: 'inline-flex',
		alignItems: 'center',
		fontSize: '0.85rem',
		fontWeight: 700,
		color: 'var(--primary)',
		cursor: 'pointer',
		textDecoration: 'none',
		transition: 'opacity 0.2s',
		alignSelf: 'flex-start',
	},
	leftContent: {
		display: 'flex',
		flexDirection: 'column' as const,
		gap: '16px',
	},
	postMeta: {
		display: 'flex',
		justifyContent: 'space-between',
		alignItems: 'center',
		paddingRight: '36px', // 접기 버튼과 겹침 방지 여백 확보
	},
	categoryBadge: {
		background: 'rgba(79, 70, 229, 0.1)',
		color: 'var(--primary)',
		fontSize: '0.8rem',
		fontWeight: 700,
		padding: '4px 12px',
		borderRadius: '12px',
	},
	postTitle: {
		fontFamily: "'Outfit', sans-serif",
		fontSize: '1.6rem',
		fontWeight: 800,
		color: 'var(--text-main)',
		lineHeight: 1.3,
	},
	postAuthor: {
		fontSize: '0.85rem',
		color: 'var(--text-muted)',
	},
	divider: {
		border: 'none',
		borderTop: '1px solid var(--border-color)',
		margin: '8px 0',
	},
	postBody: {
		fontSize: '1rem',
		lineHeight: 1.7,
		color: 'var(--text-main)',
		whiteSpace: 'pre-wrap' as const,
	},
	rightPanel: {
		display: 'flex',
		flexDirection: 'column' as const,
		flex: 1,
		background: 'rgba(0, 0, 0, 0.01)',
	},
	chatHeader: {
		padding: '16px 24px',
		borderBottom: '1px solid var(--border-color)',
		display: 'flex',
		justifyContent: 'space-between',
		alignItems: 'center',
		background: 'var(--panel-bg)',
	},
	chatTitle: {
		fontSize: '0.95rem',
		fontWeight: 700,
		color: 'var(--text-main)',
	},
	chatMetaZone: {
		display: 'flex',
		alignItems: 'center',
		gap: '12px',
	},
	activeUsers: {
		fontSize: '0.8rem',
		fontWeight: 600,
		color: 'var(--text-muted)',
	},
	stateBadge: {
		fontSize: '0.75rem',
		fontWeight: 800,
		padding: '4px 10px',
		borderRadius: '10px',
		letterSpacing: '0.5px',
	},
	chatTimeline: {
		flex: 1,
		padding: '24px',
		overflowY: 'auto' as const,
		display: 'flex',
		flexDirection: 'column' as const,
		gap: '16px',
	},
	emptyChat: {
		flex: 1,
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		color: 'var(--text-muted)',
		fontSize: '0.9rem',
	},
	chatBubbleRow: {
		display: 'flex',
		flexDirection: 'column' as const,
		alignItems: 'flex-start',
		maxWidth: '80%',
	},
	replyContext: {
		fontSize: '0.75rem',
		color: 'var(--text-muted)',
		marginBottom: '4px',
		marginLeft: '12px',
	},
	bubbleMain: {
		display: 'flex',
		flexDirection: 'column' as const,
		gap: '4px',
	},
	senderName: {
		fontSize: '0.75rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		marginLeft: '12px',
	},
	bubbleText: {
		background: 'var(--chat-other-bg)',
		border: '1px solid var(--border-color)',
		color: 'var(--text-main)',
		padding: '10px 16px',
		borderRadius: '16px',
		fontSize: '0.9rem',
		lineHeight: 1.4,
		wordBreak: 'break-all' as const,
		cursor: 'pointer',
		boxShadow: 'var(--shadow-sm)',
		transition: 'border-color 0.2s',
		'&:hover': {
			borderColor: 'var(--primary)',
		}
	},
	reactionBar: {
		padding: '12px 24px',
		borderTop: '1px solid var(--border-color)',
		borderBottom: '1px solid var(--border-color)',
		display: 'flex',
		gap: '10px',
		background: 'var(--panel-bg)',
	},
	reactBtn: {
		background: 'var(--bg-color)',
		border: '1px solid var(--border-color)',
		borderRadius: '20px',
		padding: '6px 14px',
		display: 'flex',
		alignItems: 'center',
		gap: '8px',
		fontSize: '0.85rem',
		fontWeight: 600,
		color: 'var(--text-main)',
		boxShadow: 'var(--shadow-sm)',
		transition: 'transform 0.15s',
		outline: 'none',
	},
	reactEmoji: {
		fontSize: '1rem',
	},
	reactCount: {
		fontFamily: "'Outfit', sans-serif",
	},
	replyBanner: {
		padding: '8px 24px',
		background: 'rgba(79, 70, 229, 0.05)',
		borderTop: '1px solid var(--border-color)',
		display: 'flex',
		justifyContent: 'space-between',
		alignItems: 'center',
		fontSize: '0.8rem',
		color: 'var(--primary)',
	},
	replyCancelBtn: {
		background: 'none',
		border: 'none',
		fontSize: '1.25rem',
		color: 'var(--text-muted)',
		cursor: 'pointer',
	},
	chatForm: {
		padding: '16px 24px',
		display: 'flex',
		gap: '12px',
		background: 'var(--panel-bg)',
		borderTop: '1px solid var(--border-color)',
	},
	chatInput: {
		flex: 1,
		background: 'var(--bg-color)',
		border: '1px solid var(--border-color)',
		borderRadius: '24px',
		padding: '12px 20px',
		color: 'var(--text-main)',
		fontSize: '0.9rem',
		outline: 'none',
	},
	sendBtn: {
		background: 'var(--primary)',
		border: 'none',
		borderRadius: '24px',
		color: 'white',
		padding: '0 24px',
		fontWeight: 700,
		fontSize: '0.9rem',
		cursor: 'pointer',
		transition: 'background 0.2s',
	},
	lockedBanner: {
		padding: '20px 24px',
		textAlign: 'center' as const,
		background: 'rgba(0, 0, 0, 0.03)',
		borderTop: '1px solid var(--border-color)',
		color: 'var(--text-muted)',
		fontSize: '0.85rem',
		fontWeight: 600,
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		gap: '8px',
	},
};
