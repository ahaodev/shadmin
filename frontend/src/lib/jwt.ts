/**
 * Decode a JWT token payload without verification.
 * Returns the payload as a record, or null if decoding fails.
 */
export function decodeJwt(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split('.')
    if (parts.length < 2) return null
    const base64Url = parts[1]
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const pad = base64.length % 4
    const padded = base64 + (pad ? '='.repeat(4 - pad) : '')
    const json = decodeURIComponent(
      atob(padded)
        .split('')
        .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    )
    return JSON.parse(json)
  } catch (e) {
    console.error('Failed to decode JWT', e)
    return null
  }
}

/**
 * Build an AuthUser from a JWT token string.
 * Returns null if the token cannot be decoded.
 */
export function authUserFromJwt(token: string): {
  accountNo: string
  email: string
  role: string[]
  exp: number
} | null {
  const payload = decodeJwt(token)
  if (!payload) return null

  return {
    accountNo: String(payload.name ?? payload.username ?? payload.id ?? ''),
    email: String(payload.email ?? ''),
    role: Array.isArray(payload.role)
      ? (payload.role as string[])
      : ['authenticated_user'],
    exp:
      typeof payload.exp === 'number'
        ? payload.exp
        : Date.now() + 24 * 60 * 60 * 1000,
  }
}
