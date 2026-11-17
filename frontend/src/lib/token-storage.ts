/** Shared by auth-store and interceptors to avoid circular imports. */
import { ACCESS_TOKEN, REFRESH_TOKEN } from '@/types/constants'

// Scope tokens by identity so stale refreshes cannot overwrite another account.
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
  localStorage.removeItem('access_token')
  removeCookie(ACCESS_TOKEN)
}

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

export function removeAllTokens() {
  removeAccessToken()
  removeRefreshToken()
  localStorage.removeItem('userProfile')
  localStorage.removeItem('userPermissions')
  localStorage.removeItem('lastLoginTime')
}
