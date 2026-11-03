import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { session } from '@/lib/session'
import {
  getAccessToken,
  getRefreshToken,
  removeAllTokens,
} from '@/lib/token-storage'

export const getApiBaseURL = () => {
  return `${window.location.protocol}//${window.location.host}`
}

export const apiClient = axios.create({
  baseURL: getApiBaseURL(),
  headers: { 'Content-Type': 'application/json' },
  timeout: 300000,
})

// Bare axios instance for token refresh — no interceptors, avoids cycles
const refreshClient = axios.create({
  baseURL: getApiBaseURL(),
  headers: { 'Content-Type': 'application/json' },
  timeout: 15000,
})

// Auth routes that should never carry an Authorization header or trigger refresh
const AUTH_ROUTES = [
  '/api/v1/auth/login',
  '/api/v1/auth/refresh',
  '/api/v1/auth/captcha/slide',
  '/api/v1/auth/device/code',
  '/api/v1/auth/device/token',
  '/api/v1/auth/identity/providers',
  '/api/v1/auth/identity/exchange',
  '/api/v1/auth/invitations/accept',
]
function isAuthRoute(url?: string): boolean {
  return AUTH_ROUTES.some((r) => url?.includes(r))
}

// --- Request interceptor: attach access token ---
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const request = config as RetryableConfig
    request._sessionVersion ??= session.version
    if (!session.isCurrent(request._sessionVersion)) {
      throw new axios.CanceledError('Session changed')
    }
    config.signal = config.signal
      ? AbortSignal.any([config.signal as AbortSignal, session.signal])
      : session.signal
    if (!isAuthRoute(config.url)) {
      const token = getAccessToken()
      if (token) {
        config.headers = config.headers || {}
        config.headers['Authorization'] = `Bearer ${token}`
      }
    }
    return config
  },
  (error: unknown) => Promise.reject(error)
)

// --- Response interceptor: 401 → refresh → retry ---

// Extend AxiosRequestConfig to carry our retry flag
interface RetryableConfig extends InternalAxiosRequestConfig {
  _retry?: boolean
  _sessionVersion?: number
}

let isLoggingOut = false
let refreshFlight: { version: number; promise: Promise<string> } | null = null

function forceLogout() {
  if (isLoggingOut) return
  isLoggingOut = true

  authHandlers?.reset()
  removeAllTokens()

  const currentPath = window.location.pathname + window.location.search
  if (!currentPath.startsWith('/sign-in')) {
    const redirect = encodeURIComponent(
      window.location.pathname + window.location.search + window.location.hash
    )
    window.location.replace(`/sign-in?redirect=${redirect}`)
  }

  // Reset flag after navigation starts
  setTimeout(() => {
    isLoggingOut = false
  }, 1000)
}

interface AuthHandlers {
  reset: () => void
  setTokens: (accessToken: string, refreshToken?: string) => void
}
let authHandlers: AuthHandlers | null = null
export function registerAuthHandlers(handlers: AuthHandlers) {
  authHandlers = handlers
}

function assertCurrentRequest(config: RetryableConfig | undefined) {
  if (
    config?._sessionVersion !== undefined &&
    !session.isCurrent(config._sessionVersion)
  ) {
    throw new axios.CanceledError('Session changed')
  }
}

async function refreshAccessToken(version: number): Promise<string> {
  if (refreshFlight?.version === version) return refreshFlight.promise

  const pending = (async () => {
    const refreshToken = getRefreshToken()
    if (!refreshToken) {
      forceLogout()
      throw new Error('No refresh token')
    }
    try {
      const response = await refreshClient.post(
        '/api/v1/auth/refresh',
        { refreshToken },
        { signal: session.signal }
      )
      if (!session.isCurrent(version))
        throw new axios.CanceledError('Session changed')
      const data = response.data
      if (data?.code !== 0 || !data?.data?.accessToken) {
        throw new Error('Invalid refresh response')
      }
      authHandlers?.setTokens(data.data.accessToken, data.data.refreshToken)
      return data.data.accessToken as string
    } catch (error) {
      const status = axios.isAxiosError(error)
        ? error.response?.status
        : undefined
      if (
        session.isCurrent(version) &&
        !axios.isCancel(error) &&
        (!status || [400, 401, 403].includes(status))
      )
        forceLogout()
      throw error
    }
  })()
  refreshFlight = { version, promise: pending }
  try {
    return await pending
  } finally {
    if (refreshFlight?.promise === pending) refreshFlight = null
  }
}

apiClient.interceptors.response.use(
  (response) => {
    assertCurrentRequest(response.config)
    return response
  },
  async (error: AxiosError) => {
    const originalRequest = error.config as RetryableConfig | undefined
    assertCurrentRequest(originalRequest)
    if (
      error.response?.status !== 401 ||
      !originalRequest ||
      originalRequest._retry ||
      isAuthRoute(originalRequest.url)
    ) {
      return Promise.reject(error)
    }

    originalRequest._retry = true
    const token = await refreshAccessToken(
      originalRequest._sessionVersion ?? session.version
    )
    assertCurrentRequest(originalRequest)
    originalRequest.headers['Authorization'] = `Bearer ${token}`
    return apiClient(originalRequest)
  }
)
