import { request } from '@/api/client'
import { toSearchParams } from '@/api/pagination'
import type { PageData } from '@/types/common'
import type {
  Conversation,
  CreateConversationRequest,
  Message,
  QuestionAccepted,
  QuestionInput,
} from './types'

export function createConversation(kbId: string, data: CreateConversationRequest): Promise<Conversation> {
  return request<Conversation>(`/knowledge-bases/${kbId}/conversations`, {
    method: 'POST',
    body: data,
  })
}

export function listConversations(kbId: string, query?: { page?: number; page_size?: number }): Promise<PageData<Conversation>> {
  const params = toSearchParams(query)
  return request<PageData<Conversation>>(`/knowledge-bases/${kbId}/conversations?${params.toString()}`)
}

export function getConversation(id: string): Promise<Conversation> {
  return request<Conversation>(`/conversations/${id}`)
}

export function deleteConversation(id: string): Promise<void> {
  return request<void>(`/conversations/${id}`, {
    method: 'DELETE',
  })
}

export function getMessages(conversationId: string, query?: { page?: number; page_size?: number }): Promise<PageData<Message>> {
  const params = toSearchParams(query)
  return request<PageData<Message>>(`/conversations/${conversationId}/messages?${params.toString()}`)
}

export function submitQuestion(conversationId: string, data: QuestionInput): Promise<QuestionAccepted> {
  return request<QuestionAccepted>(`/conversations/${conversationId}/questions`, {
    method: 'POST',
    body: data,
  })
}
