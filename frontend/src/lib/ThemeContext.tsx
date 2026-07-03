'use client';

import React, { createContext, useContext, useEffect, useState } from 'react';

type Theme = 'light' | 'dark';

interface ThemeContextType {
	theme: Theme;
	toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
	const [theme, setTheme] = useState<Theme>('light');
	const [mounted, setMounted] = useState(false);

	useEffect(() => {
		// 최초 마운트 시 브라우저 로컬스토리지에서 테마 조회
		const savedTheme = localStorage.getItem('theme') as Theme;
		if (savedTheme) {
			setTheme(savedTheme);
			document.documentElement.setAttribute('data-theme', savedTheme);
		} else {
			// 디폴트는 라이트 모드
			document.documentElement.setAttribute('data-theme', 'light');
		}
		setMounted(true);
	}, []);

	const toggleTheme = () => {
		const nextTheme = theme === 'light' ? 'dark' : 'light';
		setTheme(nextTheme);
		localStorage.setItem('theme', nextTheme);
		document.documentElement.setAttribute('data-theme', nextTheme);
	};

	// 하이드레이션 오류 방지를 위해 마운트 완료 전까지 뼈대 상태로 서빙
	return (
		<ThemeContext.Provider value={{ theme, toggleTheme }}>
			{children}
		</ThemeContext.Provider>
	);
}

export function useTheme() {
	const context = useContext(ThemeContext);
	if (context === undefined) {
		throw new Error('useTheme must be used within a ThemeProvider');
	}
	return context;
}
