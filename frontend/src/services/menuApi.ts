import {
  type CreateMenuRequest,
  type Menu,
  type MenuPagedResult,
  type MenuQueryParams,
  type MenuTreeNode,
  type UpdateMenuRequest,
} from '@/types/menu'
import { buildSearchParams } from '@/lib/query-params'
import { apiClient } from './config'

export const getMenus = async (
  params?: MenuQueryParams
): Promise<MenuPagedResult> => {
  const searchParams = buildSearchParams(params)

  const response = await apiClient.get(
    `/api/v1/system/menu?${searchParams.toString()}`
  )
  return response.data.data
}

export const createMenu = async (request: CreateMenuRequest): Promise<Menu> => {
  const response = await apiClient.post('/api/v1/system/menu', request)
  return response.data.data
}

export const getMenuTree = async (status?: string): Promise<MenuTreeNode[]> => {
  const params = new URLSearchParams()
  if (status) params.append('status', status)

  const response = await apiClient.get(
    `/api/v1/system/menu/tree?${params.toString()}`
  )
  return response.data.data
}

export const updateMenu = async (
  id: string,
  request: UpdateMenuRequest
): Promise<Menu> => {
  const response = await apiClient.put(`/api/v1/system/menu/${id}`, request)
  return response.data.data
}

export const deleteMenu = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/menu/${id}`)
}
