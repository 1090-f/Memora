import { apiRequest } from '@/api/client';
import type { PageResult } from '@/features/knowledge-base/types';
import type { Conversation, Message, QuestionResponse } from './types';

export const listConversations = (kbId: string, params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<Conversation>>({ url: `/knowledge-bases/${kbId}/conversations`, params });

export const createConversation = (kbId: string, title: string) =>
  apiRequest<Conversation>({ url: `/knowledge-bases/${kbId}/conversations`, method: 'POST', data: { title } });

export const getConversation = (id: string) =>
  apiRequest<Conversation>({ url: `/conversations/${id}` });

export const listMessages = (id: string, params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<Message>>({ url: `/conversations/${id}/messages`, params });

export const deleteConversation = (id: string) =>
  apiRequest<void>({ url: `/conversations/${id}`, method: 'DELETE' });

export const submitQuestion = (id: string, query: string) =>
  apiRequest<QuestionResponse>({ url: `/conversations/${id}/questions`, method: 'POST', data: { query } });
