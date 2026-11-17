import { useEffect, useMemo, useState } from 'react'
import type { SidebarData } from '@/types/layout'
import { Command } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { menuService } from '@/lib/menu-service'

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

/** Reads cached sidebar data without fetching. */
export function getSidebarData(): SidebarData {
  return {
    user: resolveUserInfo(),
    teams: defaultSidebarData.teams,
    navGroups: menuService.getCachedMenuData() || [],
  }
}

export function useSidebarData() {
  const [sidebarData, setSidebarData] = useState<SidebarData>(getSidebarData())
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const { auth } = useAuthStore()
  // 权限单独变更时也需刷新侧边栏。
  const userKey = useMemo(
    () =>
      JSON.stringify({
        accountNo: auth.user?.accountNo,
        email: auth.user?.email,
        profileId: auth.profile?.id,
        profileEmail: auth.profile?.email,
        profileUsername: auth.profile?.username,
        profileAvatar: auth.profile?.avatar,
        accessToken: auth.accessToken ?? null,
        permissions: auth.permissions,
      }),
    [
      auth.user?.accountNo,
      auth.user?.email,
      auth.profile?.id,
      auth.profile?.email,
      auth.profile?.username,
      auth.profile?.avatar,
      auth.accessToken,
      auth.permissions,
    ]
  )

  useEffect(() => {
    let mounted = true

    const loadData = async () => {
      if (!useAuthStore.getState().auth.accessToken) {
        return
      }

      try {
        setIsLoading(true)
        setError(null)

        const user = resolveUserInfo()
        const navGroups = await useAuthStore.getState().auth.loadMenus()

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
  }, [userKey])

  const reloadData = async () => {
    if (!useAuthStore.getState().auth.accessToken) {
      return
    }

    try {
      setIsLoading(true)
      setError(null)

      const user = resolveUserInfo()
      const navGroups = await useAuthStore.getState().auth.loadMenus(true)

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
