'use client';

import React, { useState } from 'react';
import { createPostAPI } from '../lib/api';

interface CreatePostModalProps {
	isOpen: boolean;
	onClose: () => void;
	onSuccess: () => void;
}

export default function CreatePostModal({ isOpen, onClose, onSuccess }: CreatePostModalProps) {
	const [title, setTitle] = useState('');
	const [content, setContent] = useState('');
	const [category, setCategory] = useState('자유');
	const [loading, setLoading] = useState(false);
	const [errorMsg, setErrorMsg] = useState('');

	if (!isOpen) return null;

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!title.trim()) {
			setErrorMsg('제목을 입력해주세요.');
			return;
		}

		setLoading(true);
		setErrorMsg('');

		try {
			await createPostAPI({
				title: title.trim(),
				content: content.trim(),
				category,
				image_urls: [],
			});
			setTitle('');
			setContent('');
			onSuccess();
			onClose();
		} catch (err: any) {
			setErrorMsg(err.message || '게시글 작성 중 오류가 발생했습니다.');
		} finally {
			setLoading(false);
		}
	};

	return (
		<div style={styles.overlay} onClick={onClose}>
			<div style={styles.modal} onClick={e => e.stopPropagation()}>
				<div style={styles.header}>
					<h2 style={styles.title}>새 게시글 작성</h2>
					<button style={styles.closeBtn} onClick={onClose}>&times;</button>
				</div>

				<form onSubmit={handleSubmit} style={styles.form}>
					{errorMsg && (
						<div style={styles.errorBanner}>
							⚠️ {errorMsg}
						</div>
					)}

					<div style={styles.formGroup}>
						<label style={styles.label}>카테고리</label>
						<select 
							value={category} 
							onChange={e => setCategory(e.target.value)}
							style={styles.select}
						>
							<option value="자유">자유</option>
							<option value="경제·주식">경제·주식</option>
							<option value="스포츠">스포츠</option>
							<option value="연예·문화">연예·문화</option>
						</select>
					</div>

					<div style={styles.formGroup}>
						<label style={styles.label}>제목 (100자 이내)</label>
						<input 
							type="text" 
							value={title}
							onChange={e => setTitle(e.target.value)}
							placeholder="제목을 입력하세요"
							maxLength={100}
							style={styles.input}
							disabled={loading}
						/>
					</div>

					<div style={styles.formGroup}>
						<label style={styles.label}>본문 내용</label>
						<textarea 
							value={content}
							onChange={e => setContent(e.target.value)}
							placeholder="여기에 생각이나 의견을 나누어 보세요 (최대 3000자)"
							maxLength={3000}
							rows={6}
							style={styles.textarea}
							disabled={loading}
						/>
					</div>

					<div style={styles.footer}>
						<button 
							type="button" 
							onClick={onClose} 
							style={styles.cancelBtn}
							disabled={loading}
						>
							취소
						</button>
						<button 
							type="submit" 
							style={styles.submitBtn}
							disabled={loading}
						>
							{loading ? '등록 중...' : '작성 완료'}
						</button>
					</div>
				</form>
			</div>
		</div>
	);
}

const styles = {
	overlay: {
		position: 'fixed' as const,
		top: 0,
		left: 0,
		right: 0,
		bottom: 0,
		zIndex: 1000,
		background: 'rgba(0, 0, 0, 0.4)',
		backdropFilter: 'blur(8px)',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'center',
		padding: '20px',
	},
	modal: {
		width: '100%',
		maxWidth: '600px',
		background: 'var(--bg-color)',
		border: '1px solid var(--border-color)',
		borderRadius: '16px',
		boxShadow: 'var(--shadow-lg)',
		overflow: 'hidden',
		animation: 'fadeIn 0.3s ease',
	},
	header: {
		padding: '20px 24px',
		borderBottom: '1px solid var(--border-color)',
		display: 'flex',
		alignItems: 'center',
		justifyContent: 'space-between',
	},
	title: {
		fontFamily: "'Outfit', sans-serif",
		fontSize: '1.25rem',
		fontWeight: 700,
		color: 'var(--text-main)',
	},
	closeBtn: {
		background: 'none',
		border: 'none',
		fontSize: '1.75rem',
		color: 'var(--text-muted)',
		cursor: 'pointer',
		lineHeight: 1,
	},
	form: {
		padding: '24px',
	},
	errorBanner: {
		padding: '12px 16px',
		borderRadius: '8px',
		background: 'rgba(244, 63, 94, 0.1)',
		border: '1px solid rgba(244, 63, 94, 0.2)',
		color: 'var(--accent)',
		fontSize: '0.9rem',
		fontWeight: 600,
		marginBottom: 20,
	},
	formGroup: {
		marginBottom: '20px',
	},
	label: {
		display: 'block',
		fontSize: '0.875rem',
		fontWeight: 600,
		color: 'var(--text-muted)',
		marginBottom: '8px',
	},
	input: {
		width: '100%',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '8px',
		padding: '12px 16px',
		color: 'var(--text-main)',
		fontSize: '0.95rem',
		outline: 'none',
		transition: 'border-color 0.2s',
	},
	select: {
		width: '100%',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '8px',
		padding: '12px 16px',
		color: 'var(--text-main)',
		fontSize: '0.95rem',
		outline: 'none',
		transition: 'border-color 0.2s',
	},
	textarea: {
		width: '100%',
		background: 'var(--panel-bg)',
		border: '1px solid var(--border-color)',
		borderRadius: '8px',
		padding: '12px 16px',
		color: 'var(--text-main)',
		fontSize: '0.95rem',
		outline: 'none',
		resize: 'vertical' as const,
		transition: 'border-color 0.2s',
	},
	footer: {
		display: 'flex',
		justifyContent: 'flex-end',
		gap: '12px',
		marginTop: '32px',
	},
	cancelBtn: {
		background: 'none',
		border: '1px solid var(--border-color)',
		borderRadius: '8px',
		padding: '10px 20px',
		fontSize: '0.95rem',
		fontWeight: 600,
		color: 'var(--text-muted)',
		cursor: 'pointer',
		transition: 'background 0.2s',
	},
	submitBtn: {
		background: 'var(--primary)',
		border: 'none',
		borderRadius: '8px',
		padding: '10px 20px',
		fontSize: '0.95rem',
		fontWeight: 600,
		color: 'white',
		cursor: 'pointer',
		transition: 'background 0.2s',
	},
};
