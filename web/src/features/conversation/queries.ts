import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import {
  createConversation,
  listConversations,
  getConversation,
  deleteConversation,
  getMessages,
  submitQuestion,
} from './api'
import { conversationKeys } from './types'
import type { CreateConversationRequest, QuestionInput } from './types'

export function useConversationList(kbId: () => string) {
  return useQuery({
    queryKey: computed(() => conversationKeys.list(kbId())),
    queryFn: () => listConversations(kbId()),
    enabled: computed(() => !!kbId()),
  })
}

export function useConversationDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => conversationKeys.detail(id())),
    queryFn: () => getConversation(id()),
    enabled: computed(() => !!id()),
  })
}

export function useCreateConversation(kbId: () => string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateConversationRequest) => createConversation(kbId(), data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: conversationKeys.list(kbId()) })
    },
  })
}

export function useDeleteConversation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteConversation(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['conversations'] })
    },
  })
}

export function useMessageList(conversationId: () => string) {
  return useQuery({
    queryKey: computed(() => conversationKeys.messages(conversationId())),
    queryFn: () => getMessages(conversationId(), { page: 1, page_size: 100 }),
    enabled: computed(() => !!conversationId()),
  })
}

export function useSubmitQuestion(conversationId: () => string) {
  return useMutation({
    mutationFn: (data: QuestionInput) => submitQuestion(conversationId(), data),
  })
}
