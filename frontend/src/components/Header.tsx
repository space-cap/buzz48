'use client';

import React, { useEffect, useState } from 'react';
import Link from 'next/link';
import { useTheme } from '../lib/ThemeContext';
import { createSessionAPI, patchNicknameAPI, getMySessionAPI } from '../lib/api';

export default function Header() {
	const { theme, toggleTheme } = useTheme();
	const [mounted, setMounted] = useState(false);
	const [nickname, setNickname] = useState('');
	const [sessionID, setSessionID] = useState('');

	useEffect(() => {
		setMounted(true);
		initSession();

		// 다른 곳에서 닉네임 변경 시 싱크를 맞추기 위한 이벤트 모니터링
		const handleStorageChange = () => {
			setNickname(localStorage.getItem('nickname') || '');
		};
		window.addEventListener('storage', handleStorageChange);
		// 커스텀 이벤트
		window.addEventListener('nicknameChanged', handleStorageChange);

		return () => {
			window.removeEventListener('storage', handleStorageChange);
			window.removeEventListener('nicknameChanged', handleStorageChange);
		};
	}, []);

	// 세션 초기화 및 자동 발급
	const initSession = async () => {
		let savedSessionID = localStorage.getItem('session_id');
		let savedNickname = localStorage.getItem('nickname');

		if (savedSessionID) {
			try {
				// 기존 세션 검증 및 최신 닉네임 동기화
				const info = await getMySessionAPI();
				savedNickname = info.nickname;
				localStorage.setItem('nickname', info.nickname);
			} catch (err) {
				// 만료되었거나 에러 시 세션 초기화 및 재생성
				console.warn('기존 세션 만료, 세션 재생성 시도...');
				localStorage.removeItem('session_id');
				localStorage.removeItem('nickname');
				savedSessionID = null;
			}
		}

		if (!savedSessionID) {
			try {
				const session = await createSessionAPI();
				savedSessionID = session.session_id;
				savedNickname = session.nickname;
			} catch (err) {
				console.error('자동 세션 생성 실패:', err);
			}
		}

		setSessionID(savedSessionID || '');
		setNickname(savedNickname || '');
	};

	// 닉네임 수동 변경
	const handleNicknameEdit = async () => {
		const newNick = prompt('변경할 닉네임을 입력하세요 (1~50자):', nickname);
		if (!newNick || newNick.trim() === nickname) return;

		try {
			const updated = await patchNicknameAPI(newNick.trim());
			setNickname(updated.nickname);
			// 닉네임 변경 사실 전역 전파
			window.dispatchEvent(new Event('nicknameChanged'));
			alert('닉네임이 성공적으로 변경되었습니다!');
		} catch (err: any) {
			alert(err.message || '닉네임 변경에 실패했습니다.');
		}
	};

	return (
		<header style={styles.header}>
			<div style={styles.container}>
				{/* 로고 */}
				<Link href="/" style={styles.logo}>
					BUZZ48
				</Link>

				{/* 우측 유틸리티 */}
				<div style={styles.utility}>
					{mounted && nickname && (
						<div 
							onClick={handleNicknameEdit} 
							style={styles.profileBadge}
							title="닉네임 변경하기"
						>
							<span style={styles.avatar}>👤</span>
							<span style={styles.nickname}>{nickname}</span>
						</div>
					)}

					{/* 테마 토글 버튼 */}
					{mounted && (
						<button 
							onClick={toggleTheme} 
							style={styles.themeBtn}
							title={theme === 'light' ? '다크 모드로 전환' : '라이트 모드로 전환'}
						>
							{theme === 'light' ? '🌙' : '☀️'}
						</button>
					)}
				</div>
			</div>
		</header>
	);
}

const styles = {
	header: {
		position: 'sticky' as const,
		top: 0,
		zIndex: 100,
		background: 'var(--panel-bg)',
		borderBottom: '1px solid var(--border-color)',
		backdropFilter: 'blur(16px)',
		WebkitBackdropFilter: 'blur(16px)',
		boxShadow: 'var(--shadow-sm)',
		transition: 'background var(--transition-speed) ease, border-color var(--transition-speed) ease',
	},
	container: {
		maxWidth: '1200px',
		margin: '0 auto',
		padding: '0 20px',
		height: '64px',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'space-between',
	},
	logo: {
		fontFamily: "'Outfit', sans-serif",
		fontSize: '1.5rem',
		fontWeight: 800,
		letterSpacing: '-0.5px',
		background: 'linear-gradient(to right, #4f46e5, #ec4899)',
		WebkitBackgroundClip: 'text',
		WebkitTextFillColor: 'transparent',
		cursor: 'pointer',
	},
	utility: {
		display: 'flex',
		alignItems: 'center',
		gap: '16px',
	},
	profileBadge: {
		display: 'flex',
		alignItems: 'center',
		gap: '8px',
		padding: '6px 14px',
		borderRadius: '20px',
		background: 'rgba(0, 0, 0, 0.03)',
		border: '1px solid var(--border-color)',
		cursor: 'pointer',
		fontSize: '0.85rem',
		fontWeight: 600,
		color: 'var(--text-main)',
		transition: 'background 0.2s',
		userSelect: 'none' as const,
	},
	avatar: {
		fontSize: '1rem',
	},
	nickname: {
		maxWidth: '120px',
		overflow: 'hidden',
		textOverflow: 'ellipsis',
		whiteSpace: 'nowrap' as const,
	},
	themeBtn: {
		background: 'rgba(0, 0, 0, 0.05)',
		border: '1px solid var(--border-color)',
		borderRadius: '50%',
		width: '40px',
		height: '40px',
		cursor: 'pointer',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		fontSize: '1.2rem',
		transition: 'background 0.2s, transform 0.2s',
		outline: 'none',
	},
};
