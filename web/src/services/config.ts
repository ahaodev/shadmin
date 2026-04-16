import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import {
  getAccessToken,
  getRefreshToken,
  setAccessToken,
  setRefreshToken,
  removeAllTokens,
} from '@/lib/token-storage'

const getApiBaseURL = () => {
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
const AUTH_ROUTES = ['/api/v1/auth/login', '/api/v1/auth/refresh']
function isAuthRoute(url?: string): boolean {
  return AUTH_ROUTES.some((r) => url?.includes(r))
}

// --- Request interceptor: attach access token ---
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
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
}

let isRefreshing = false
let isLoggingOut = false
let failedQueue: Array<{
  resolve: (token: string) => void
  reject: (error: unknown) => void
}> = []

function processQueue(error: unknown, token: string | null) {
  failedQueue.forEach(({ resolve, reject }) => {
    if (token) resolve(token)
    else reject(error)
  })
  failedQueue = []
}

function forceLogout() {
  if (isLoggingOut) return
  isLoggingOut = true

  // Clear all auth data — import-cycle-free via token-storage
  removeAllTokens()

  // Reset zustand store lazily to avoid circular import
  try {
    // Dynamic import would be async; instead we use a post-init hook (see below)
    _resetAuthStore?.()
  } catch {
    // store may not be initialised yet
  }

  // Clear menu cache from localStorage directly
  try {
    const keys = Object.keys(localStorage)
    keys.forEach((k) => {
      if (k.startsWith('menu_') || k.startsWith('sidebar_')) {
        localStorage.removeItem(k)
      }
    })
  } catch {
    // ignore
  }

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

// Hook for auth-store to register its reset function (avoids circular import)
let _resetAuthStore: (() => void) | null = null
export function registerAuthStoreReset(fn: () => void) {
  _resetAuthStore = fn
}

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as RetryableConfig | undefined

    // Only handle 401, skip auth routes and already-retried requests
    if (
      error.response?.status !== 401 ||
      !originalRequest ||
      originalRequest._retry ||
      isAuthRoute(originalRequest.url)
    ) {
      return Promise.reject(error)
    }

    // If a refresh is already in flight, queue this request
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        failedQueue.push({
          resolve: (token: string) => {
            originalRequest._retry = true
            originalRequest.headers['Authorization'] = `Bearer ${token}`
            resolve(apiClient(originalRequest))
          },
          reject,
        })
      })
    }

    originalRequest._retry = true
    isRefreshing = true

    const currentRefreshToken = getRefreshToken()

    if (!currentRefreshToken) {
      isRefreshing = false
      processQueue(error, null)
      forceLogout()
      return Promise.reject(error)
    }

    try {
      const resp = await refreshClient.post('/api/v1/auth/refresh', {
        refreshToken: currentRefreshToken,
      })

      const data = resp.data
      if (data?.code === 0 && data?.data) {
        const newAccess: string = data.data.accessToken
        const newRefresh: string = data.data.refreshToken

        setAccessToken(newAccess)
        if (newRefresh) setRefreshToken(newRefresh)

        processQueue(null, newAccess)

        originalRequest.headers['Authorization'] = `Bearer ${newAccess}`
        return apiClient(originalRequest)
      } else {
        throw new Error('Refresh response invalid')
      }
    } catch (refreshError: unknown) {
      processQueue(refreshError, null)

      // Only force logout on auth-related refresh failures (401/400/403)
      const status =
        refreshError instanceof axios.AxiosError
          ? refreshError.response?.status
          : undefined
      if (!status || [400, 401, 403].includes(status)) {
        forceLogout()
      }
      return Promise.reject(refreshError)
    } finally {
      isRefreshing = false
    }
  }
)

// Export as api for consistency with other service files
export const api = apiClient
