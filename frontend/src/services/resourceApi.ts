import { apiClient } from '@/services/config'
import type { ResourcesResponse } from '@/types/menu'

export async function getResources(): Promise<ResourcesResponse> {
  const response = await apiClient.get('/api/v1/resources')
  return (
    response.data?.data ?? {
      menus: [],
      permissions: null,
      roles: [],
      is_admin: false,
    }
  )
}
