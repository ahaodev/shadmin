import { useQuery, useQueryClient } from '@tanstack/react-query'
import { clearAllLoginLogs, getLoginLogs } from '@/services/loginLogApi'
import type { LoginLogFilter } from '@/types/login-log'
import { useCrudMutation } from '@/hooks/use-crud-mutation'

const LOGIN_LOGS_QUERY_KEY = 'login-logs'

export function useLoginLogs(params?: LoginLogFilter) {
  return useQuery({
    queryKey: [LOGIN_LOGS_QUERY_KEY, params],
    queryFn: () => getLoginLogs(params),
    staleTime: 30 * 1000,
  })
}

export function useClearAllLoginLogs() {
  return useCrudMutation({
    mutationFn: clearAllLoginLogs,
    queryKeys: [[LOGIN_LOGS_QUERY_KEY]],
    successMessage: '已清空所有登录日志',
    errorMessage: '清空登录日志失败',
  })
}

export function useRefreshLoginLogs() {
  const queryClient = useQueryClient()

  return () => {
    queryClient.invalidateQueries({ queryKey: [LOGIN_LOGS_QUERY_KEY] })
  }
}
