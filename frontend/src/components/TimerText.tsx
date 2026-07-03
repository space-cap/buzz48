'use client';

import React, { useEffect, useState } from 'react';

interface TimerTextProps {
	initialSeconds: number;
	state: 'LIVE' | 'READ';
	onExpire?: () => void;
}

// 미니멀 SVG 시계 아이콘 (AI 이모지 대체)
const ClockIcon = ({ size = 13, style }: { size?: number; style?: React.CSSProperties }) => (
	<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" style={{ display: 'inline-block', verticalAlign: 'middle', ...style }}>
		<circle cx="12" cy="12" r="10"></circle>
		<polyline points="12 6 12 12 16 14"></polyline>
	</svg>
);

export default function TimerText({ initialSeconds, state, onExpire }: TimerTextProps) {
	const [seconds, setSeconds] = useState(initialSeconds);

	useEffect(() => {
		setSeconds(initialSeconds);
	}, [initialSeconds]);

	useEffect(() => {
		if (seconds <= 0) return;

		const timer = setInterval(() => {
			setSeconds(prev => {
				if (prev <= 1) {
					clearInterval(timer);
					if (onExpire) onExpire();
					return 0;
				}
				return prev - 1;
			});
		}, 1000);

		return () => clearInterval(timer);
	}, [seconds, onExpire]);

	const formatTime = (secs: number) => {
		const h = Math.floor(secs / 3600);
		const m = Math.floor((secs % 3600) / 60);
		const s = secs % 60;

		const hh = h < 10 ? `0${h}` : h;
		const mm = m < 10 ? `0${m}` : m;
		const ss = s < 10 ? `0${s}` : s;

		return `${hh}:${mm}:${ss}`;
	};

	const isUrgent = seconds < 3600;
	const badgeColor = state === 'LIVE' ? 'var(--green)' : 'var(--text-muted)';
	const timerColor = isUrgent ? 'var(--accent)' : 'var(--text-main)';

	return (
		<div style={styles.container}>
			{/* 상태 펄스 배지 */}
			<div style={styles.badgeZone}>
				<span style={{
					...styles.pulseDot,
					backgroundColor: badgeColor,
					boxShadow: state === 'LIVE' ? '0 0 6px var(--green)' : 'none',
				}} />
				<span style={{ ...styles.badgeText, color: badgeColor }}>
					{state}
				</span>
			</div>

			{/* 실시간 타이머 */}
			<span style={{ ...styles.timer, color: timerColor }}>
				<ClockIcon size={12} style={{ marginRight: '4px', color: timerColor }} />
				{formatTime(seconds)}
			</span>
		</div>
	);
}

const styles = {
	container: {
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'flex-end', // 테이블 우측 정렬선과 완벽 결합되도록 끝으로 밀착
		gap: '10px',
		fontSize: '0.8rem',
		fontWeight: 700,
		width: '100%',
		lineHeight: 1,
	},
	badgeZone: {
		display: 'inline-flex',
		alignItems: 'center',
		gap: '5px',
		lineHeight: 1,
	},
	pulseDot: {
		width: '5px',
		height: '5px',
		borderRadius: '50%',
		display: 'inline-block',
	},
	badgeText: {
		letterSpacing: '0.5px',
		fontSize: '0.72rem',
		fontWeight: 800,
		lineHeight: 1,
	},
	timer: {
		fontFamily: "'Outfit', sans-serif",
		letterSpacing: '0.2px',
		display: 'inline-flex',
		alignItems: 'center',
		lineHeight: 1,
	},
};
