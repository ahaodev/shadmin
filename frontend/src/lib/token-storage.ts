/**
 * Dependency-free token storage helpers.
 * Used by both auth-store and axios interceptors to avoid circular imports.
 *
 * 令牌只存一处（localStorage）。旧版本同时写一份非 HttpOnly cookie：JS 可读写，
 * 因此并不提供额外的 XSS 防护，只是把同一份凭证的暴露面翻倍；
 * 后端只从 Authorization 头取令牌，从不读取该 cookie。
 * 启动时清理旧格式凭证并要求重新登录，不把未绑定身份的 token 迁入当前会话。
 */
import { ACCESS_TOKEN, REFRESH_TOKEN } from '@/types/constants'

// Tokens are scoped by the shared identity marker. An old tab finishing refresh
// after an account switch can write only its old namespace, never the new account.
function tokenKey(
  name: string,
  identity = localStorage.getItem('shadmin-session')
) {
  return identity ? `${name}:${identity}` : null
}

function removeCookie(name: string) {
  document.cookie = `${name}=; path=/; max-age=0`
}

export function removeLegacyTokens() {
  localStorage.removeItem(ACCESS_TOKEN)
  localStorage.removeItem('access_token')
  localStorage.removeItem(REFRESH_TOKEN)
  removeCookie(ACCESS_TOKEN)
  removeCookie(REFRESH_TOKEN)
}

// --- Access Token ---

export function getAccessToken(): string {
  const key = tokenKey(ACCESS_TOKEN)
  return (key && localStorage.getItem(key)) || ''
}

export function setAccessToken(token: string, identity?: string | null) {
  const key = tokenKey(ACCESS_TOKEN, identity)
  if (key) localStorage.setItem(key, token)
}

export function removeAccessToken() {
  const key = tokenKey(ACCESS_TOKEN)
  if (key) localStorage.removeItem(key)
  localStorage.removeItem(ACCESS_TOKEN)
  // Also clean legacy key used in login form
  localStorage.removeItem('access_token')
  removeCookie(ACCESS_TOKEN)
}

// --- Refresh Token ---

export function getRefreshToken(): string {
  const key = tokenKey(REFRESH_TOKEN)
  return (key && localStorage.getItem(key)) || ''
}

export function setRefreshToken(token: string, identity?: string | null) {
  const key = tokenKey(REFRESH_TOKEN, identity)
  if (key) localStorage.setItem(key, token)
}

export function removeRefreshToken() {
  const key = tokenKey(REFRESH_TOKEN)
  if (key) localStorage.removeItem(key)
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
