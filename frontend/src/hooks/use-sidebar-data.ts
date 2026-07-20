import { useEffect, useMemo, useState } from 'react'
import { menuService } from '@/services/menu-service'
import { Command } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { type SidebarData } from '@/components/layout/types'

const defaultSidebarData = {
  user: {
    name: 'User',
    email: 'user@example.com',
    avatar: '/avatars/shadcn.jpg',
  },
  teams: [
    {
      name: 'shadmin',
      logo: Command,
      plan: 'Vite + ShadcnUI',
    },
  ],
}

/**
 * Build sidebar user info from auth-store, falling back to defaults when no
 * user is signed in.
 */
function resolveUserInfo() {
  const auth = useAuthStore.getState().auth

  if (auth.profile) {
    return {
      name: auth.profile.username || auth.profile.email || 'User',
      email: auth.profile.email || '',
      avatar: auth.profile.avatar || '/avatars/shadcn.jpg',
    }
  }

  if (auth.user) {
    return {
      name: auth.user.email || auth.user.accountNo || 'User',
      email: auth.user.email || '',
      avatar: '/avatars/shadcn.jpg',
    }
  }

  console.warn('No authenticated user found, using default user data')
  return defaultSidebarData.user
}

/**
 * Get sidebar data synchronously from cached state.
 * Used by command-menu and as initial hook state.
 */
export function getSidebarData(): SidebarData {
  return {
    user: resolveUserInfo(),
    teams: defaultSidebarData.teams,
    navGroups: menuService.getCachedMenuData() || [],
  }
}

// ---------------------------------------------------------------------------
// React hook
// ---------------------------------------------------------------------------

export function useSidebarData() {
  const [sidebarData, setSidebarData] = useState<SidebarData>(getSidebarData())
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // 监听用户状态变化
  const { auth } = useAuthStore()
  // 创建更稳定的用户标识符，避免每次渲染重复 JSON.stringify。
  // 包含 permissions 以便仅权限变更（setPermissions）时也能刷新侧边栏。
  const userKey = useMemo(
    () =>
      JSON.stringify({
        accountNo: auth.user?.accountNo,
        email: auth.user?.email,
        profileId: auth.profile?.id,
        profileEmail: auth.profile?.email,
        accessToken: auth.accessToken ?? null,
        permissions: auth.permissions,
      }),
    [
      auth.user?.accountNo,
      auth.user?.email,
      auth.profile?.id,
      auth.profile?.email,
      auth.accessToken,
      auth.permissions,
    ]
  )

  useEffect(() => {
    let mounted = true

    const loadData = async () => {
      // Skip loading if user is not authenticated (e.g. during logout)
      if (!useAuthStore.getState().auth.accessToken) {
        return
      }

      try {
        setIsLoading(true)
        setError(null)

        const user = resolveUserInfo()
        const navGroups = await menuService.loadMenuData()

        if (mounted) {
          setSidebarData({
            user,
            teams: defaultSidebarData.teams,
            navGroups: navGroups || [],
          })
        }
      } catch (err) {
        if (mounted) {
          setError(
            err instanceof Error ? err.message : 'Failed to load menu data'
          )
          console.error('Failed to load sidebar data:', err)
          // Keep the current/fallback data on error
        }
      } finally {
        if (mounted) {
          setIsLoading(false)
        }
      }
    }

    loadData()

    return () => {
      mounted = false
    }
  }, [userKey]) // 当用户关键信息改变时重新加载数据

  const reloadData = async () => {
    // Skip reloading if user is not authenticated (e.g. after logout)
    if (!useAuthStore.getState().auth.accessToken) {
      return
    }

    try {
      setIsLoading(true)
      setError(null)

      menuService.clearCache()

      const user = resolveUserInfo()
      const navGroups = await menuService.loadMenuData()

      setSidebarData({
        user,
        teams: defaultSidebarData.teams,
        navGroups: navGroups || [],
      })
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to reload menu data'
      )
      console.error('Manual reload failed:', err)
    } finally {
      setIsLoading(false)
    }
  }

  return {
    sidebarData,
    isLoading,
    error,
    reloadData,
  }
}
