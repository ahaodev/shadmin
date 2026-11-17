import { apiClient } from '@/services/config'
import type {
  PaginatedLoginLogsResponse,
  LoginLogFilter,
} from '@/types/login-log'
import { buildSearchParams } from '@/lib/query-params'

export async function getLoginLogs(
  params?: LoginLogFilter
): Promise<PaginatedLoginLogsResponse> {
  const searchParams = buildSearchParams(params)

  const response = await apiClient.get(
    `/api/v1/system/login-logs?${searchParams}`
  )
  return response.data.data
}

export async function clearAllLoginLogs(): Promise<string> {
  const response = await apiClient.delete('/api/v1/system/login-logs')
  return response.data.data
}
