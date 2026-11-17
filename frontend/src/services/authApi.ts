import { apiClient, getApiBaseURL } from '@/services/config'
import { type ApiResponse } from '@/types/api'
import { getRefreshToken } from '@/lib/token-storage'

export interface LoginRequest {
  identifier: string
  password: string
  captcha_id: string
  captcha_x: number
  captcha_y: number
}

export interface SlideCaptchaChallenge {
  captcha_id: string
  master_image: string
  tile_image: string
  tile_x: number
  tile_y: number
  tile_width: number
  tile_height: number
  master_width: number
  master_height: number
  expires_in: number
}

export interface LoginResponse {
  accessToken: string
  refreshToken: string
}

export interface DeviceActivateRequest {
  user_code: string
}

export interface DeviceActivateResponse {
  status: string
}

export async function login(
  credentials: LoginRequest
): Promise<ApiResponse<LoginResponse>> {
  const resp = await apiClient.post('/api/v1/auth/login', credentials)
  return resp.data
}

export async function getSlideCaptcha(
  oldCaptchaId?: string
): Promise<ApiResponse<SlideCaptchaChallenge>> {
  const params = oldCaptchaId ? { old_captcha_id: oldCaptchaId } : undefined
  const resp = await apiClient.get('/api/v1/auth/captcha/slide', { params })
  return resp.data
}

export async function activateDevice(
  request: DeviceActivateRequest
): Promise<ApiResponse<DeviceActivateResponse>> {
  const resp = await apiClient.post('/api/v1/auth/device/activate', request)
  return resp.data
}

const USER_IDENTITY_LOGIN_BASE_PATH = '/api/v1/auth/identity'

export function getIdentityLoginHref(provider: string): string {
  return new URL(
    `${USER_IDENTITY_LOGIN_BASE_PATH}/${provider}`,
    getApiBaseURL()
  ).toString()
}

export async function getIdentityProviders(): Promise<ApiResponse<string[]>> {
  const resp = await apiClient.get('/api/v1/auth/identity/providers')
  return resp.data
}

export async function exchangeUserIdentityCode(
  code: string
): Promise<ApiResponse<LoginResponse>> {
  const resp = await apiClient.post('/api/v1/auth/identity/exchange', { code })
  return resp.data
}

export async function logout(): Promise<ApiResponse<void>> {
  const refreshToken = getRefreshToken()

  const requestBody = refreshToken ? { refresh_token: refreshToken } : {}

  const resp = await apiClient.post('/api/v1/auth/logout', requestBody)
  return resp.data
}
