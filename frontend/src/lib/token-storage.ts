/**
 * Dependency-free token storage helpers.
 * Used by both auth-store and axios interceptors to avoid circular imports.
 *
 * 令牌只存一处（localStorage）。旧版本同时写一份非 HttpOnly cookie：JS 可读写，
 * 因此并不提供额外的 XSS 防护，只是把同一份凭证的暴露面翻倍；
 * 后端只从 Authorization 头取令牌，从不读取该 cookie。
 * remove* 仍会清理旧版本留下的同名 cookie。
 */
import { ACCESS_TOKEN, REFRESH_TOKEN } from '@/types/constants'

function removeCookie(name: string) {
  document.cookie = `${name}=; path=/; max-age=0`
}

// --- Access Token ---

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_TOKEN) || ''
}

export function setAccessToken(token: string) {
  localStorage.setItem(ACCESS_TOKEN, token)
}

export function removeAccessToken() {
  localStorage.removeItem(ACCESS_TOKEN)
  // Also clean legacy key used in login form
  localStorage.removeItem('access_token')
  removeCookie(ACCESS_TOKEN)
}

// --- Refresh Token ---

export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_TOKEN) || ''
}

export function setRefreshToken(token: string) {
  localStorage.setItem(REFRESH_TOKEN, token)
}

export function removeRefreshToken() {
  localStorage.removeItem(REFRESH_TOKEN)
  removeCookie(REFRESH_TOKEN)
}

// --- Bulk cleanup ---

export function removeAllTokens() {
  removeAccessToken()
  removeRefreshToken()
  localStorage.removeItem('userProfile')
  localStorage.removeItem('userPermissions')
  localStorage.removeItem('lastLoginTime')
}
