import { apiClient } from '@/services/config'
import {
  type Profile,
  type ProfileUpdateRequest,
  type PasswordUpdateRequest,
} from '@/types/profile'

export async function getProfile(): Promise<Profile> {
  const response = await apiClient.get('/api/v1/profile')
  return response.data.data
}

export async function updateProfile(
  data: ProfileUpdateRequest
): Promise<string> {
  const response = await apiClient.put('/api/v1/profile', data)
  return response.data.data
}

export async function updatePassword(
  data: PasswordUpdateRequest
): Promise<string> {
  const response = await apiClient.put('/api/v1/profile/password', data)
  return response.data.data
}
