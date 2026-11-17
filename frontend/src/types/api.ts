export interface ApiResponse<T> {
  code: number // 0 successful
  msg: string
  data: T
}

export interface ApiError {
  response?: {
    data?: {
      msg?: string
    }
  }
}

export interface PagedResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface QueryParams {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

export type UserPagedResult = PagedResult<import('./user').User>
