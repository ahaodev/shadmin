import { apiClient } from '@/services/config'
import { type UserPagedResult, type QueryParams } from '@/types/api'
import {
  type User,
  type CreateUserRequest,
  type InviteUserRequest,
  type AcceptInvitationRequest,
  type UserUpdateRequest,
} from '@/types/user'
import { buildSearchParams } from '@/lib/query-params'

export async function getUsers(params?: QueryParams): Promise<UserPagedResult> {
  const searchParams = buildSearchParams(params)

  const response = await apiClient.get(`/api/v1/system/user?${searchParams}`)
  return response.data.data
}

export async function createUser(data: CreateUserRequest): Promise<User> {
  const response = await apiClient.post('/api/v1/system/user', data)
  return response.data.data
}

export async function inviteUser(data: InviteUserRequest): Promise<User> {
  const response = await apiClient.post('/api/v1/system/user/invite', data)
  return response.data.data
}

export async function acceptUserInvitation(
  data: AcceptInvitationRequest
): Promise<void> {
  await apiClient.post('/api/v1/auth/invitations/accept', data)
}

export async function getUser(id: string): Promise<User> {
  const response = await apiClient.get(`/api/v1/system/user/${id}`)
  return response.data.data
}

export async function updateUser(
  id: string,
  data: UserUpdateRequest
): Promise<User> {
  const response = await apiClient.put(`/api/v1/system/user/${id}`, data)
  return response.data.data
}

export async function deleteUser(id: string): Promise<string> {
  const response = await apiClient.delete(`/api/v1/system/user/${id}`)
  return response.data.data
}

export async function getUserRoleList(userId: string): Promise<string[]> {
  const response = await apiClient.get(`/api/v1/system/user/${userId}/roles`)
  const data = response.data?.data

  return Array.isArray(data) ? data : []
}
