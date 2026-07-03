import type { Metadata, Viewport } from "next";
import "./globals.css";
import { ThemeProvider } from "../lib/ThemeContext";
import Header from "../components/Header";

export const metadata: Metadata = {
	title: "버즈48 (Buzz48) — 24시간 생존형 실시간 게시판",
	description: "24시간만 활성화되고 48시간 뒤 완전 영구 파기되는 실시간 타임라인 커뮤니티",
};

export const viewport: Viewport = {
	width: "device-width",
	initialScale: 1,
	maximumScale: 1,
};

export default function RootLayout({
	children,
}: Readonly<{
	children: React.ReactNode;
}>) {
	return (
		<html lang="ko" suppressHydrationWarning>
			<head>
				{/* 렌더링 즉시 로컬스토리지의 테마 속성을 주입하여 흰색 깜빡임 현상 완벽 방어 */}
				<script
					dangerouslySetInnerHTML={{
						__html: `
							(function() {
								try {
									var savedTheme = localStorage.getItem('theme') || 'light';
									document.documentElement.setAttribute('data-theme', savedTheme);
								} catch (e) {
									document.documentElement.setAttribute('data-theme', 'light');
								}
							})();
						`,
					}}
				/>
			</head>
			<body>
				<ThemeProvider>
					<Header />
					<main style={styles.main}>
						{children}
					</main>
				</ThemeProvider>
			</body>
		</html>
	);
}

const styles = {
	main: {
		flex: 1,
		width: '100%',
		maxWidth: '1200px',
		margin: '0 auto',
		padding: '24px 20px',
	},
};
