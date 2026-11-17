import { type QueryParams } from '@/types/api'
import {
  type CreateRoleRequest,
  type Role,
  type RoleInfo,
  type RolePagedResult,
  type UpdateRoleRequest,
} from '@/types/role'
import { apiClient } from './config'

export const getRoles = async (): Promise<RoleInfo[]> => {
  const response = await apiClient.get(`/api/v1/system/role`)
  return response.data.data
}

export const createRole = async (request: CreateRoleRequest): Promise<Role> => {
  const response = await apiClient.post('/api/v1/system/role', request)
  return response.data.data
}

export const updateRole = async (
  id: string,
  request: UpdateRoleRequest
): Promise<Role> => {
  const response = await apiClient.put(`/api/v1/system/role/${id}`, request)
  return response.data.data
}

export const deleteRole = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/role/${id}`)
}

export const getRolesPaged = async (
  params?: QueryParams
): Promise<RolePagedResult> => {
  // The role API is unpaginated; paginate locally.
  const roles = await getRoles()

  const page = params?.page || 1
  const pageSize = params?.page_size || 10
  const startIndex = (page - 1) * pageSize
  const endIndex = startIndex + pageSize

  const paginatedRoles = roles.slice(startIndex, endIndex)

  const roleList: Role[] = paginatedRoles.map((role) => ({
    id: role.id,
    name: role.name,
    sequence: role.sequence ?? 0,
    status: role.status ?? 'active',
    created_at: new Date(),
    updated_at: new Date(),
    menu_ids: role.menus,
  }))

  return {
    list: roleList,
    total: roles.length,
    page,
    page_size: pageSize,
    total_pages: Math.ceil(roles.length / pageSize),
  }
}

export const getRoleMenus = async (roleId: string): Promise<string[]> => {
  const response = await apiClient.get(`/api/v1/system/role/${roleId}/menus`)
  return response.data.data
}
