import type { PageData, PageQuery } from '@/types/common'

export type { PageData, PageQuery }

export function toSearchParams(query?: PageQuery): URLSearchParams {
  const params = new URLSearchParams()
  if (query?.page !== undefined) params.set('page', String(query.page))
  if (query?.page_size !== undefined) params.set('page_size', String(query.page_size))
  if (query?.keyword) params.set('keyword', query.keyword)
  if (query?.sort) params.set('sort', query.sort)
  return params
}
