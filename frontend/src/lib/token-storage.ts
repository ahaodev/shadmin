/**
 * Dependency-free token storage helpers.
 * Used by both auth-store and axios interceptors to avoid circular imports.
 */
import { ACCESS_TOKEN, REFRESH_TOKEN } from '@/types/constants'

const COOKIE_MAX_AGE = 60 * 60 * 24 * 7 // 7 days

function setCookie(name: string, value: string, maxAge = COOKIE_MAX_AGE) {
  document.cookie = `${name}=${value}; path=/; max-age=${maxAge}`
}

function getCookie(name: string): string | undefined {
  const value = `; ${document.cookie}`
  const parts = value.split(`; ${name}=`)
  if (parts.length === 2) return parts.pop()?.split(';').shift()
  return undefined
}

function removeCookie(name: string) {
  document.cookie = `${name}=; path=/; max-age=0`
}

// --- Access Token ---

export function getAccessToken(): string {
  return getCookie(ACCESS_TOKEN) || localStorage.getItem(ACCESS_TOKEN) || ''
}

export function setAccessToken(token: string) {
  setCookie(ACCESS_TOKEN, token)
  localStorage.setItem(ACCESS_TOKEN, token)
}

export function removeAccessToken() {
  removeCookie(ACCESS_TOKEN)
  localStorage.removeItem(ACCESS_TOKEN)
  // Also clean legacy key used in login form
  localStorage.removeItem('access_token')
}

// --- Refresh Token ---

export function getRefreshToken(): string {
  return getCookie(REFRESH_TOKEN) || localStorage.getItem(REFRESH_TOKEN) || ''
}

export function setRefreshToken(token: string) {
  setCookie(REFRESH_TOKEN, token)
  localStorage.setItem(REFRESH_TOKEN, token)
}

export function removeRefreshToken() {
  removeCookie(REFRESH_TOKEN)
  localStorage.removeItem(REFRESH_TOKEN)
}

// --- Bulk cleanup ---

export function removeAllTokens() {
  removeAccessToken()
  removeRefreshToken()
  localStorage.removeItem('userProfile')
  localStorage.removeItem('userPermissions')
  localStorage.removeItem('lastLoginTime')
}
