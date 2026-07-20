import { apiClient, getApiBaseURL } from '@/services/config.ts'
import { type ApiResponse } from '@/types/api.ts'
import { type Profile } from '@/types/profile.ts'
import { type User } from '@/types/user'
import { getRefreshToken } from '@/lib/token-storage'

// 登录请求类型
export interface LoginRequest {
  username: string
  password: string
  captcha_id: string
  captcha_x: number
  captcha_y: number
}

// Slide 验证码挑战
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

// 登录响应类型
export interface LoginResponse {
  user: User
  accessToken: string
  refreshToken: string
  providerAvatarUrl?: string
}

// 刷新令牌响应类型
export interface RefreshTokenResponse {
  accessToken: string
  refreshToken: string
}

export interface DeviceCodeRequest {
  client_id: string
  client_name?: string
}

export interface DeviceCodeResponse {
  device_code: string
  user_code: string
  verification_uri: string
  expires_in: number
  interval: number
}

export interface DeviceTokenRequest {
  client_id: string
  device_code: string
}

export interface DeviceActivateRequest {
  user_code: string
}

export interface DeviceActivateResponse {
  status: string
}

// 用户登录
export async function login(
  credentials: LoginRequest
): Promise<ApiResponse<LoginResponse>> {
  const resp = await apiClient.post('/api/v1/auth/login', credentials)
  return resp.data
}

// 获取/刷新 Slide 验证码挑战
export async function getSlideCaptcha(
  oldCaptchaId?: string
): Promise<ApiResponse<SlideCaptchaChallenge>> {
  const params = oldCaptchaId ? { old_captcha_id: oldCaptchaId } : undefined
  const resp = await apiClient.get('/api/v1/auth/captcha/slide', { params })
  return resp.data
}

export async function requestDeviceCode(
  request: DeviceCodeRequest
): Promise<ApiResponse<DeviceCodeResponse>> {
  const resp = await apiClient.post('/api/v1/auth/device/code', request)
  return resp.data
}

export async function pollDeviceToken(
  request: DeviceTokenRequest
): Promise<ApiResponse<LoginResponse>> {
  const resp = await apiClient.post('/api/v1/auth/device/token', request)
  return resp.data
}

export async function activateDevice(
  request: DeviceActivateRequest
): Promise<ApiResponse<DeviceActivateResponse>> {
  const resp = await apiClient.post('/api/v1/auth/device/activate', request)
  return resp.data
}

const USER_IDENTITY_LOGIN_BASE_PATH = '/api/v1/auth/identity'

export function getIdentityLoginHref(
  provider: string
): string {
  return new URL(
    `${USER_IDENTITY_LOGIN_BASE_PATH}/${provider}`,
    getApiBaseURL()
  ).toString()
}

// 获取后端当前已启用的第三方登录 provider 列表
export async function getIdentityProviders(): Promise<
  ApiResponse<string[]>
> {
  const resp = await apiClient.get('/api/v1/auth/identity/providers')
  return resp.data
}

// 用一次性 code 交换第三方登录的 JWT 令牌
export async function exchangeUserIdentityCode(
  code: string
): Promise<ApiResponse<LoginResponse>> {
  const resp = await apiClient.post('/api/v1/auth/identity/exchange', { code })
  return resp.data
}

// 刷新访问令牌
export async function refreshToken(
  refreshToken: string
): Promise<ApiResponse<RefreshTokenResponse>> {
  const resp = await apiClient.post('/api/v1/auth/refresh', {
    refreshToken: refreshToken,
  })
  return resp.data
}

// 登出
export async function logout(): Promise<ApiResponse<void>> {
  // 使用 token-storage 获取 refresh token，保持存储策略一致性
  const refreshToken = getRefreshToken()

  const requestBody = refreshToken ? { refresh_token: refreshToken } : {}

  const resp = await apiClient.post('/api/v1/auth/logout', requestBody)
  return resp.data
}

// 验证当前令牌是否有效
export async function validateToken(): Promise<ApiResponse<Profile>> {
  const resp = await apiClient.get('/api/v1/profile')
  return resp.data
}
