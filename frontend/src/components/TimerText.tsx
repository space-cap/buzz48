'use client';

import React, { useEffect, useState } from 'react';

interface TimerTextProps {
	initialSeconds: number;
	state: 'LIVE' | 'READ';
	onExpire?: () => void;
}

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

	// 1시간(3600초) 미만인지 감지
	const isUrgent = seconds < 3600;

	// 배지 및 상태별 스타일 분기
	const badgeColor = state === 'LIVE' ? 'var(--green)' : 'var(--text-muted)';
	const timerColor = isUrgent ? 'var(--accent)' : 'var(--text-main)';

	return (
		<div style={styles.container}>
			{/* 상태 펄스 배지 */}
			<div style={styles.badgeZone}>
				<span style={{
					...styles.pulseDot,
					backgroundColor: badgeColor,
					boxShadow: state === 'LIVE' ? '0 0 8px var(--green)' : 'none',
					animation: state === 'LIVE' ? 'pulse 2s infinite' : 'none',
				}} />
				<span style={{ ...styles.badgeText, color: badgeColor }}>
					{state}
				</span>
			</div>

			{/* 실시간 타이머 */}
			<span style={{ ...styles.timer, color: timerColor }}>
				⏱️ {formatTime(seconds)}
			</span>
		</div>
	);
}

const styles = {
	container: {
		display: 'flex',
		alignItems: 'center',
		gap: '12px',
		fontSize: '0.85rem',
		fontWeight: 600,
	},
	badgeZone: {
		display: 'inline-flex',
		alignItems: 'center',
		gap: '6px',
	},
	pulseDot: {
		width: '6px',
		height: '6px',
		borderRadius: '50%',
		display: 'inline-block',
	},
	badgeText: {
		letterSpacing: '0.5px',
		fontSize: '0.75rem',
	},
	timer: {
		fontFamily: "'Outfit', sans-serif",
		letterSpacing: '0.2px',
	},
};
