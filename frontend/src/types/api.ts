// 后端统一返回类型
export interface ApiResponse<T> {
  code: number // 响应码，例如 0 表示成功
  msg: string // 响应消息，例如 "ok"
  data: T // 泛型数据，可以是任意类型
}

// 分页结果类型 (与后端 domain.PagedResult 对应)
export interface PagedResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

// 查询参数类型
export interface QueryParams {
  page?: number
  page_size?: number
  status?: string
  keyword?: string
}

// 用户分页结果
export type UserPagedResult = PagedResult<import('./user').User>
