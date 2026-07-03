import React from 'react';
import type { Metadata, Viewport } from 'next';
import { getPostDetailAPI } from '../../../lib/api';
import PostDetailClient from './PostDetailClient';

interface Props {
	params: Promise<{ id: string }>;
}

// 1. Next.js 서버 사이드 동적 OG 메타 태그 렌더링 (PRD 및 SEO 규정)
export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const resolvedParams = await params;
	const postID = resolvedParams.id;

	try {
		// 서버사이드 프리페치 시도
		const post = await getPostDetailAPI(postID);
		const remainingHours = Math.max(0, Math.ceil(post.remaining_seconds / 3600));

		return {
			title: `버즈48 | ${post.title}`,
			description: `⏱️ 파기 임박! 앞으로 약 ${remainingHours}시간 후 이 게시물은 우주 먼지로 영구 삭제됩니다.`,
			openGraph: {
				title: `버즈48 — ${post.title}`,
				description: `⏱️ 파기 임박! 앞으로 약 ${remainingHours}시간 후 흔적도 없이 완전 영구 파기됩니다. 실시간 대화에 참여하세요!`,
				type: 'article',
				url: `http://localhost:3001/posts/${postID}`,
			},
		};
	} catch (err) {
		return {
			title: "버즈48 — 소멸되었거나 존재하지 않는 게시글",
			description: "이 게시물은 파기 제한 시간(48시간)이 경과하여 완전히 영구 삭제되었습니다.",
		};
	}
}

export const viewport: Viewport = {
	width: "device-width",
	initialScale: 1,
	maximumScale: 1,
};

// 2. 서버 컴포넌트 메인 렌더러
export default async function Page({ params }: Props) {
	const resolvedParams = await params;
	const postID = resolvedParams.id;

	return <PostDetailClient postID={postID} />;
}
