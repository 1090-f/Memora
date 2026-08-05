export interface ApiEnvelope<T> {
  code: string
  message: string
  data: T
  request_id: string
}

export interface PageData<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export interface PageQuery {
  page?: number
  page_size?: number
  keyword?: string
  sort?: string
}
