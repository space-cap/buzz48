'use client';

import React, { useEffect, useState, useCallback, useRef } from 'react';
import Link from 'next/link';
import { getPostsAPI, getHotPostsAPI, PostItem, HotPostItem } from '../lib/api';
import TimerText from '../components/TimerText';
import CreatePostModal from '../components/CreatePostModal';

export default function Home() {
	// 상태 관리
	const [activeCategory, setActiveCategory] = useState('전체');
	const [posts, setPosts] = useState<PostItem[]>([]);
	const [hotPosts, setHotPosts] = useState<HotPostItem[]>([]);
	const [nextCursor, setNextCursor] = useState<string | null>(null);
	const [loading, setLoading] = useState(false);
	const [isWriteModalOpen, setIsWriteModalOpen] = useState(false);

	// 검색 상태 관리
	const [searchKeyword, setSearchKeyword] = useState('');
	const [searchQuery, setSearchQuery] = useState('');
	const [isSearchFocused, setIsSearchFocused] = useState(false);

	// 무한 스크롤 감지용 Ref
	const loadMoreTriggerRef = useRef<HTMLDivElement | null>(null);

	// 카테고리 정의
	const categories = ['전체', '자유', '경제·주식', '스포츠', '연예·문화'];

	// 1. 최신 게시글 목록 조회
	const fetchPosts = useCallback(async (category: string, append = false, cursor = '', searchVal = '') => {
		setLoading(true);
		try {
			const catParam = category === '전체' ? undefined : category;
			const data = await getPostsAPI({ 
				category: catParam, 
				cursor, 
				search: searchVal || undefined,
				limit: 15 
			});
			
			if (append) {
				setPosts(prev => [...prev, ...data.posts]);
			} else {
				setPosts(data.posts);
			}
			setNextCursor(data.next_cursor);
		} catch (err) {
			console.error('피드 로딩 오류:', err);
		} finally {
			setLoading(false);
		}
	}, []);

	// 2. 실시간 HOT 3 조회
	const fetchHotPosts = useCallback(async () => {
		try {
			const data = await getHotPostsAPI();
			setHotPosts(data.posts.slice(0, 3)); // 상위 3개만 제한
		} catch (err) {
			console.error('HOT 피드 로딩 오류:', err);
		}
	}, []);

	// 초기 마운트, 카테고리 전환, 검색 실행 시 실행
	useEffect(() => {
		fetchPosts(activeCategory, false, '', searchQuery);
		fetchHotPosts();

		// HOT 3 게시글은 10초 주기로 실시간 폴링 갱신
		const hotTimer = setInterval(() => {
			fetchHotPosts();
		}, 10*1000);

		// 닉네임 변경 시 세션 연동 확인용
		const handleNickChange = () => {
			fetchPosts(activeCategory, false, '', searchQuery);
		};
		window.addEventListener('nicknameChanged', handleNickChange);

		return () => {
			clearInterval(hotTimer);
			window.removeEventListener('nicknameChanged', handleNickChange);
		};
	}, [activeCategory, searchQuery, fetchPosts, fetchHotPosts]);

	// 더보기 페이징 처리
	const handleLoadMore = useCallback(() => {
		if (nextCursor && !loading) {
			fetchPosts(activeCategory, true, nextCursor, searchQuery);
		}
	}, [nextCursor, loading, activeCategory, searchQuery, fetchPosts]);

	// 자동 무한 스크롤 옵저버 설정
	useEffect(() => {
		if (!nextCursor || loading) return;

		const observer = new IntersectionObserver((entries) => {
			if (entries[0].isIntersecting) {
				handleLoadMore();
			}
		}, {
			threshold: 0.1,
		});

		const currentTrigger = loadMoreTriggerRef.current;
		if (currentTrigger) {
			observer.observe(currentTrigger);
		}

		return () => {
			if (currentTrigger) {
				observer.unobserve(currentTrigger);
			}
		};
	}, [nextCursor, loading, handleLoadMore]);

	// 검색 핸들러
	const handleSearchSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		setSearchQuery(searchKeyword.trim());
	};

	const handleSearchReset = () => {
		setSearchKeyword('');
		setSearchQuery('');
	};

	// 카테고리 변경 핸들러 (검색 상태 리셋 추가)
	const handleCategoryChange = (cat: string) => {
		setSearchKeyword('');
		setSearchQuery('');
		setActiveCategory(cat);
	};

	// 게시글 작성 성공 시 리프레시
	const handlePostSuccess = () => {
		fetchPosts(activeCategory, false, '', searchQuery);
		fetchHotPosts();
	};

	return (
		<div style={styles.container}>
			{/* ────────────────────────────────────────────────────────
			   1. 실시간 HOT 3 하이라이트 보드
			   ──────────────────────────────────────────────────────── */}
			<section style={styles.hotSection}>
				<h2 style={styles.sectionTitle}>🔥 실시간 HOT 3 (10초마다 갱신)</h2>
				{hotPosts.length === 0 ? (
					<div style={styles.emptyHot}>인기글을 집계하고 있습니다. 잠시만 대기해 주세요.</div>
				) : (
					<div style={styles.hotGrid}>
						{hotPosts.map((post, idx) => (
							<Link href={`/posts/${post.post_id}`} key={post.post_id} style={styles.hotCardLink}>
								<div style={{
									...styles.hotCard,
									border: idx === 0 ? '1px solid var(--accent)' : '1px solid var(--border-color)'
								}}>
									{/* 등수 뱃지 */}
									<span style={{
										...styles.rankBadge,
										background: idx === 0 ? 'var(--accent)' : 'var(--primary)'
									}}>
										#{idx + 1}
									</span>
									<div style={styles.hotCategory}>{post.category}</div>
									<h3 style={styles.hotTitle}>{post.title}</h3>
									
									<div style={styles.hotFooter}>
										<div style={styles.hotMeta}>
											<span>👀 {post.conn_count}명 접속 중</span>
											<span>🔥 Score: {post.score.toFixed(1)}</span>
										</div>
									</div>
								</div>
							</Link>
						))}
					</div>
				)}
			</section>

			{/* ────────────────────────────────────────────────────────
			   2. 카테고리 탭 필터 & 목록 헤더
			   ──────────────────────────────────────────────────────── */}
			<section style={styles.feedSection}>
				<div style={styles.feedHeader}>
					{/* 카테고리 탭 리스트 */}
					<div style={styles.tabs}>
						{categories.map(cat => (
							<button
								key={cat}
								onClick={() => handleCategoryChange(cat)}
								style={{
									...styles.tabBtn,
									color: activeCategory === cat ? 'white' : 'var(--text-muted)',
									background: activeCategory === cat ? 'var(--primary)' : 'rgba(0, 0, 0, 0.03)',
									borderColor: activeCategory === cat ? 'var(--primary)' : 'var(--border-color)',
								}}
							>
								{cat}
							</button>
						))}
					</div>

					{/* 실시간 검색창 폼 */}
					<form 
						onSubmit={handleSearchSubmit} 
						style={{
							...styles.searchForm,
							borderColor: isSearchFocused ? 'var(--primary)' : 'var(--border-color)'
						}}
					>
						<span style={styles.searchIcon}>🔍</span>
						<input
							type="text"
							value={searchKeyword}
							onChange={(e) => setSearchKeyword(e.target.value)}
							onFocus={() => setIsSearchFocused(true)}
							onBlur={() => setIsSearchFocused(false)}
							placeholder="제목, 내용 키워드 검색..."
							style={styles.searchInput}
						/>
						{searchKeyword && (
							<button type="button" onClick={handleSearchReset} style={styles.searchResetBtn}>
								&times;
							</button>
						)}
						<button type="submit" style={styles.searchSubmitBtn}>검색</button>
					</form>

					<button 
						onClick={() => setIsWriteModalOpen(true)}
						style={styles.headerWriteBtn}
					>
						✍️ 새 글 쓰기
					</button>
				</div>

				{/* ────────────────────────────────────────────────────────
				   3. 게시글 컴팩트 리스트
				   ──────────────────────────────────────────────────────── */}
				{posts.length === 0 ? (
					<div style={styles.emptyFeed}>
						{loading ? '게시글을 불러오는 중입니다...' : '아직 등록된 게시글이 없습니다.'}
					</div>
				) : (
					<div style={styles.feedList}>
						{/* 리스트 헤더 */}
						<div style={styles.listHeader}>
							<span style={styles.headerCategory}>분류</span>
							<span style={styles.headerTitle}>제목</span>
							<span style={styles.headerConn}>참여</span>
							<span style={styles.headerViews}>조회</span>
							<span style={styles.headerTimer}>남은 시간</span>
						</div>
						{/* 리스트 로우 */}
						{posts.map((post, idx) => {
							const isLast = idx === posts.length - 1;
							return (
								<Link href={`/posts/${post.post_id}`} key={post.post_id} style={styles.cardLink}>
									<div style={{
										...styles.listRow,
										borderBottom: isLast ? 'none' : '1px solid var(--border-color)'
									}}>
										{/* 분류 */}
										<span style={styles.rowCategory}>{post.category}</span>

										{/* 제목 + NEW 뱃지 */}
										<span style={styles.rowTitle}>
											{post.title}
											{post.is_new && <span style={styles.newBadge}>N</span>}
										</span>

										{/* 참여자 수 */}
										<span style={styles.rowConn}>
											👀 {post.conn_count}
										</span>

										{/* 조회수 */}
										<span style={styles.rowViews}>
											🔥 {post.view_count}
										</span>

										{/* 남은 시간 타이머 */}
										<span style={styles.rowTimer}>
											<TimerText
												initialSeconds={post.remaining_seconds}
												state={post.state}
											/>
										</span>
									</div>
								</Link>
							);
						})}
					</div>
				)}

				{/* 4. 무한 스크롤 트리거 영역 */}
				{nextCursor && (
					<div ref={loadMoreTriggerRef} style={styles.loadMoreZone}>
						{loading && <span style={styles.loadMoreText}>🔄 다음 글을 가져오는 중...</span>}
					</div>
				)}
			</section>

			{/* 5. 플로팅 액션 작성 버튼 (FAB) */}
			<button 
				style={styles.fabBtn} 
				onClick={() => setIsWriteModalOpen(true)}
				title="새 게시글 작성"
			>
				+
			</button>

			{/* 6. 게시글 작성 모달 장착 */}
			<CreatePostModal 
				isOpen={isWriteModalOpen}
				onClose={() => setIsWriteModalOpen(false)}
				onSuccess={handlePostSuccess}
			/>
		</div>
	);
}

const styles = {
	container: {
		width: '100%',
		display: 'flex',
		flexDirection: 'column' as const,
		gap: '40px',
	},
	hotSection: {
		width: '100%',
	},
	sectionTitle: {
		fontFamily: "'Outfit', sans-serif",
		fontSize: '1.2rem',
		fontWeight: 700,
		marginBottom: '16px',
		color: 'var(--text-main)',
	},
	emptyHot: {
		padding: '24px',
		textAlign: 'center' as const,
		color: 'var(--text-muted)',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '12px',
		fontSize: '0.9rem',
	},
	hotGrid: {
		display: 'grid',
		gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))',
		gap: '20px',
	},
	hotCardLink: {
		textDecoration: 'none',
		color: 'inherit',
	},
	hotCard: {
		background: 'var(--panel-bg)',
		borderRadius: '16px',
		padding: '24px',
		position: 'relative' as const,
		backdropFilter: 'blur(12px)',
		WebkitBackdropFilter: 'blur(12px)',
		boxShadow: 'var(--shadow-md)',
		cursor: 'pointer',
		transition: 'transform 0.2s, box-shadow 0.2s',
		height: '160px',
		display: 'flex',
		flexDirection: 'column' as const,
		justifyContent: 'space-between',
		'&:hover': {
			transform: 'translateY(-4px)',
			boxShadow: 'var(--shadow-lg)',
		}
	},
	rankBadge: {
		position: 'absolute' as const,
		top: '16px',
		right: '16px',
		color: 'white',
		fontSize: '0.75rem',
		fontWeight: 700,
		padding: '4px 10px',
		borderRadius: '12px',
		fontFamily: "'Outfit', sans-serif",
	},
	hotCategory: {
		fontSize: '0.8rem',
		fontWeight: 600,
		color: 'var(--primary)',
		textTransform: 'uppercase' as const,
	},
	hotTitle: {
		fontSize: '1.1rem',
		fontWeight: 700,
		color: 'var(--text-main)',
		marginTop: '8px',
		marginBottom: '8px',
		lineHeight: 1.4,
		overflow: 'hidden',
		textOverflow: 'ellipsis',
		display: '-webkit-box',
		WebkitLineClamp: 2,
		WebkitBoxOrient: 'vertical' as const,
	},
	hotFooter: {
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'space-between',
		fontSize: '0.8rem',
		color: 'var(--text-muted)',
	},
	hotMeta: {
		display: 'flex',
		gap: '12px',
		fontWeight: 600,
	},
	feedSection: {
		width: '100%',
	},
	feedHeader: {
		display: 'flex',
		justifyContent: 'space-between',
		alignItems: 'center',
		marginBottom: '20px',
		flexWrap: 'wrap' as const,
		gap: '16px',
	},
	tabs: {
		display: 'flex',
		gap: '8px',
		overflowX: 'auto' as const,
		paddingBottom: '4px',
		maxWidth: '100%',
	},
	tabBtn: {
		border: '1px solid transparent',
		borderRadius: '20px',
		padding: '8px 18px',
		fontSize: '0.85rem',
		fontWeight: 600,
		cursor: 'pointer',
		transition: 'all 0.2s ease',
		whiteSpace: 'nowrap' as const,
	},
	headerWriteBtn: {
		background: 'var(--primary)',
		border: 'none',
		borderRadius: '20px',
		padding: '8px 20px',
		color: 'white',
		fontSize: '0.85rem',
		fontWeight: 700,
		cursor: 'pointer',
		boxShadow: 'var(--shadow-sm)',
		transition: 'background 0.2s',
	},
	emptyFeed: {
		padding: '80px 24px',
		textAlign: 'center' as const,
		color: 'var(--text-muted)',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '16px',
		fontSize: '0.95rem',
	},
	feedList: {
		display: 'flex',
		flexDirection: 'column' as const,
		gap: '0',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '14px',
		overflow: 'hidden',
		boxShadow: 'var(--shadow-sm)',
	},
	cardLink: {
		textDecoration: 'none',
		color: 'inherit',
		display: 'block',
	},
	// 리스트 헤더 행
	listHeader: {
		display: 'flex',
		alignItems: 'center',
		paddingTop: '10px',
		paddingBottom: '10px',
		paddingLeft: '20px',
		paddingRight: '20px',
		background: 'rgba(79, 70, 229, 0.04)',
		borderBottom: '1px solid var(--border-color)',
		gap: '16px',
	},
	listHeaderCell: {
		fontSize: '0.72rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		textTransform: 'uppercase' as const,
		letterSpacing: '0.5px',
		whiteSpace: 'nowrap' as const,
	},
	headerCategory: {
		fontSize: '0.72rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		textTransform: 'uppercase' as const,
		letterSpacing: '0.5px',
		whiteSpace: 'nowrap' as const,
		width: '80px',
		textAlign: 'center' as const,
	},
	headerTitle: {
		fontSize: '0.72rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		textTransform: 'uppercase' as const,
		letterSpacing: '0.5px',
		whiteSpace: 'nowrap' as const,
		flex: 1,
		textAlign: 'left' as const,
	},
	headerConn: {
		fontSize: '0.72rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		textTransform: 'uppercase' as const,
		letterSpacing: '0.5px',
		whiteSpace: 'nowrap' as const,
		width: '60px',
		textAlign: 'right' as const,
	},
	headerViews: {
		fontSize: '0.72rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		textTransform: 'uppercase' as const,
		letterSpacing: '0.5px',
		whiteSpace: 'nowrap' as const,
		width: '70px',
		textAlign: 'right' as const,
	},
	headerTimer: {
		fontSize: '0.72rem',
		fontWeight: 700,
		color: 'var(--text-muted)',
		textTransform: 'uppercase' as const,
		letterSpacing: '0.5px',
		whiteSpace: 'nowrap' as const,
		width: '140px',
		textAlign: 'right' as const,
	},
	// 개별 리스트 로우
	listRow: {
		display: 'flex',
		alignItems: 'center',
		paddingTop: '13px',
		paddingBottom: '13px',
		paddingLeft: '20px',
		paddingRight: '20px',
		gap: '16px',
		transition: 'background 0.15s',
		cursor: 'pointer',
		'&:hover': {
			background: 'rgba(79, 70, 229, 0.03)',
		}
	},
	rowCategory: {
		flexShrink: 0,
		width: '80px',
		fontSize: '0.75rem',
		fontWeight: 700,
		color: 'white',
		background: 'var(--primary)',
		borderRadius: '6px',
		padding: '3px 8px',
		textAlign: 'center' as const,
		whiteSpace: 'nowrap' as const,
		overflow: 'hidden',
		textOverflow: 'ellipsis',
	},
	rowTitle: {
		flex: 1,
		fontSize: '0.95rem',
		fontWeight: 600,
		color: 'var(--text-main)',
		whiteSpace: 'nowrap' as const,
		overflow: 'hidden',
		textOverflow: 'ellipsis',
		display: 'flex',
		alignItems: 'center',
		gap: '6px',
	},
	rowConn: {
		flexShrink: 0,
		width: '60px',
		textAlign: 'right' as const,
		fontSize: '0.8rem',
		fontWeight: 600,
		color: 'var(--text-muted)',
		whiteSpace: 'nowrap' as const,
	},
	rowViews: {
		flexShrink: 0,
		width: '70px',
		textAlign: 'right' as const,
		fontSize: '0.8rem',
		fontWeight: 600,
		color: 'var(--text-muted)',
		whiteSpace: 'nowrap' as const,
	},
	rowTimer: {
		flexShrink: 0,
		width: '140px',
		textAlign: 'right' as const,
		fontSize: '0.82rem',
		fontFamily: "'Outfit', monospace",
		fontWeight: 700,
		whiteSpace: 'nowrap' as const,
	},
	newBadge: {
		background: 'rgba(79, 70, 229, 0.12)',
		color: 'var(--primary)',
		fontSize: '0.6rem',
		fontWeight: 900,
		padding: '1px 5px',
		borderRadius: '4px',
		letterSpacing: '0.5px',
		flexShrink: 0,
	},
	loadMoreZone: {
		display: 'flex',
		justifyContent: 'center',
		marginTop: '24px',
		paddingBottom: '20px',
	},
	loadMoreText: {
		fontSize: '0.85rem',
		color: 'var(--text-muted)',
		padding: '8px 16px',
		background: 'rgba(0, 0, 0, 0.02)',
		borderRadius: '20px',
		display: 'inline-flex',
		alignItems: 'center',
		gap: '8px',
	},
	searchForm: {
		display: 'flex',
		alignItems: 'center',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '20px',
		padding: '6px 14px',
		flex: 1,
		maxWidth: '320px',
		boxShadow: 'var(--shadow-sm)',
		transition: 'border-color 0.2s',
	},
	searchIcon: {
		fontSize: '0.9rem',
		marginRight: '6px',
		color: 'var(--text-muted)',
		userSelect: 'none' as const,
	},
	searchInput: {
		border: 'none',
		outline: 'none',
		background: 'transparent',
		color: 'var(--text-main)',
		fontSize: '0.85rem',
		width: '100%',
		padding: '0',
	},
	searchResetBtn: {
		border: 'none',
		outline: 'none',
		background: 'transparent',
		color: 'var(--text-muted)',
		fontSize: '1.1rem',
		cursor: 'pointer',
		padding: '0 6px',
		display: 'flex',
		alignItems: 'center',
		'&:hover': {
			color: 'var(--text-main)',
		}
	},
	searchSubmitBtn: {
		border: 'none',
		outline: 'none',
		background: 'var(--primary)',
		color: 'white',
		borderRadius: '14px',
		padding: '4px 10px',
		fontSize: '0.75rem',
		fontWeight: 700,
		cursor: 'pointer',
		whiteSpace: 'nowrap' as const,
		marginLeft: '4px',
		transition: 'opacity 0.2s',
		'&:hover': {
			opacity: 0.9,
		}
	},
	fabBtn: {
		position: 'fixed' as const,
		bottom: '32px',
		right: '32px',
		width: '56px',
		height: '56px',
		borderRadius: '50%',
		background: 'var(--primary)',
		color: 'white',
		fontSize: '1.75rem',
		border: 'none',
		boxShadow: '0 4px 16px rgba(79, 70, 229, 0.3)',
		cursor: 'pointer',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		transition: 'transform 0.2s, background 0.2s',
		zIndex: 99,
		'&:hover': {
			transform: 'scale(1.05)',
		}
	},
};
