import { useAuthStore } from '@/stores/auth-store'
import { hasPermission, hasRole, canAccessMenu } from '@/lib/permissions'

export function usePermission() {
  const { permissions } = useAuthStore().auth

  return {
    hasPermission: (permission: string) =>
      hasPermission(permissions, permission),
    hasAnyPermission: (ps: string[]) =>
      ps.some((p) => hasPermission(permissions, p)),
    hasAllPermissions: (ps: string[]) =>
      ps.every((p) => hasPermission(permissions, p)),
    hasRole: (role: string) => hasRole(permissions, role),
    canAccessMenu: (menuId: string) => canAccessMenu(permissions, menuId),
  }
}
