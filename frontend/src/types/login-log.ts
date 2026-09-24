export type LoginLogStatus = 'success' | 'failed'

export type LoginLog = {
  id: string
  email?: string
  login_ip: string
  user_agent: string
  browser?: string
  os?: string
  device?: string
  status: LoginLogStatus
  source?: string
  failure_reason?: string
  login_time: Date
}

export type PaginatedLoginLogsResponse = {
  list: LoginLog[] | null
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type LoginLogFilter = {
  page?: number
  page_size?: number
  email?: string
  login_ip?: string
  status?: LoginLogStatus
  source?: string
  browser?: string
  os?: string
  start_time?: string
  end_time?: string
  sort_by?: string
  order?: 'asc' | 'desc'
}
