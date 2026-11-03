import { useQuery } from '@tanstack/react-query'
import {
  type ApiResourcePagedResult,
  type ApiResourceQueryParams,
  getApiResources,
} from '@/services/apiResourceApi'

const API_RESOURCES_QUERY_KEY = 'api-resources'

export function useApiResources(params?: ApiResourceQueryParams) {
  return useQuery<ApiResourcePagedResult>({
    queryKey: [API_RESOURCES_QUERY_KEY, params],
    queryFn: () => getApiResources(params),
    staleTime: 0,
    gcTime: 0,
    refetchOnWindowFocus: false,
    retry: 2,
    refetchOnMount: true,
    placeholderData: undefined,
  })
}
