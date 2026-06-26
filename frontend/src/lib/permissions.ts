import type { UserPermissions } from '@/stores/auth-store'

/**
 * Segment-boundary wildcard match: `system:*:*` matches `system:user:read`;
 * `*` matches a single segment, segment counts must match.
 */
export function matchWildcard(pattern: string, permission: string): boolean {
  const patternParts = pattern.split(':')
  const permParts = permission.split(':')
  if (patternParts.length !== permParts.length) return false
  return patternParts.every((part, i) => part === '*' || part === permParts[i])
}

/**
 * Normalize a permission token — backend payload may be either a single
 * string or a string[] (split by ':' for each segment). Collapse to a string.
 */
export function normalizePermissionToken(p: string | string[]): string {
  return Array.isArray(p) ? (p.length === 1 ? p[0] : p.join(':')) : p
}

export function isAdmin(
  permissions: UserPermissions | null | undefined
): boolean {
  return Boolean(permissions?.is_admin)
}

export function hasPermission(
  permissions: UserPermissions | null | undefined,
  required: string
): boolean {
  if (!permissions) return false
  if (permissions.is_admin) return true
  return permissions.permissions.some((p) => {
    const token = normalizePermissionToken(p)
    return token === required || matchWildcard(token, required)
  })
}

/**
 * Returns true if the user is assigned the given role.
 * Does NOT bypass for admins — use `isAdmin()` separately if needed.
 */
export function hasRole(
  permissions: UserPermissions | null | undefined,
  role: string
): boolean {
  if (!permissions) return false
  return permissions.roles.includes(role)
}

export function canAccessMenu(
  permissions: UserPermissions | null | undefined,
  menuId: string
): boolean {
  if (!permissions) return false
  if (permissions.is_admin) return true
  return permissions.menus.includes(menuId)
}
