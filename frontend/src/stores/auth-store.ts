import { CanceledError } from 'axios'
import { registerAuthHandlers } from '@/services/config'
import { getProfile } from '@/services/profileApi'
import type { NavGroup } from '@/types/layout'
import { type User } from '@/types/user'
import { create } from 'zustand'
import { authUserFromJwt } from '@/lib/jwt'
import { menuService } from '@/lib/menu-service'
import { session } from '@/lib/session'
import {
  getAccessToken,
  setAccessToken as persistAccessToken,
  getRefreshToken,
  setRefreshToken as persistRefreshToken,
  removeAllTokens,
} from '@/lib/token-storage'

export interface AuthUser {
  accountNo: string
  email: string
  role: string[]
  exp: number
}

export interface UserPermissions {
  permissions: string[][]
  roles: string[]
  menus: string[]
  is_admin: boolean
}

interface AuthState {
  auth: {
    user: AuthUser | null
    setUser: (user: AuthUser | null) => void
    profile: User | null
    setProfile: (profile: User | null) => void
    isLoadingProfile: boolean
    fetchProfile: (force?: boolean) => Promise<void>
    accessToken: string
    setAccessToken: (accessToken: string) => void
    resetAccessToken: () => void
    startSession: (accessToken: string, refreshToken: string) => void
    loadMenus: (reload?: boolean) => Promise<NavGroup[]>
    refreshToken: string
    setRefreshToken: (refreshToken: string) => void
    permissions: UserPermissions | null
    setPermissions: (permissions: UserPermissions | null) => void
    reset: () => void
    clearSidebarCache: () => void
  }
}

export const useAuthStore = create<AuthState>()((set, get) => {
  let profileRequestVersion = 0
  const initToken = getAccessToken()
  const initRefreshToken = getRefreshToken()

  const initUser = initToken ? authUserFromJwt(initToken) : null

  return {
    auth: {
      user: initUser,
      setUser: (user) =>
        set((state) => ({ ...state, auth: { ...state.auth, user } })),
      profile: null,
      setProfile: (profile) =>
        set((state) => ({ ...state, auth: { ...state.auth, profile } })),
      isLoadingProfile: false,
      fetchProfile: async (force = false) => {
        const currentState = get()

        // Reuse cached data unless a caller explicitly requests a refresh.
        if (
          (!force &&
            (currentState.auth.isLoadingProfile ||
              currentState.auth.profile)) ||
          !currentState.auth.accessToken
        ) {
          return
        }

        const version = session.version
        const request = ++profileRequestVersion
        try {
          set((state) => ({
            ...state,
            auth: { ...state.auth, isLoadingProfile: true },
          }))
          const profile = await getProfile()
          if (!session.isCurrent(version) || request !== profileRequestVersion)
            return
          set((state) => ({
            ...state,
            auth: {
              ...state.auth,
              profile,
              isLoadingProfile: false,
            },
          }))
        } catch (error: unknown) {
          if (!session.isCurrent(version) || request !== profileRequestVersion)
            return
          // eslint-disable-next-line no-console
          console.error('Failed to fetch profile:', error)
          set((state) => ({
            ...state,
            auth: { ...state.auth, isLoadingProfile: false },
          }))

          // Sign out if user not found (404)
          if (error && typeof error === 'object' && 'response' in error) {
            const axiosErr = error as { response?: { status?: number } }
            if (axiosErr.response?.status === 404) {
              get().auth.reset()
            }
          }
        }
      },
      accessToken: initToken,
      setAccessToken: (accessToken) =>
        set((state) => {
          persistAccessToken(accessToken)
          return { ...state, auth: { ...state.auth, accessToken } }
        }),
      resetAccessToken: () => get().auth.reset(),
      startSession: (accessToken, refreshToken) => {
        get().auth.reset()
        persistAccessToken(accessToken)
        persistRefreshToken(refreshToken)
        set((state) => ({
          auth: {
            ...state.auth,
            accessToken,
            refreshToken,
            user: authUserFromJwt(accessToken),
          },
        }))
      },
      loadMenus: async (reload = false) => {
        const version = session.version
        if (reload) {
          menuService.clearCache()
          get().auth.setPermissions(null)
        }
        const menus = await menuService.loadMenuData()
        if (!session.isCurrent(version))
          throw new CanceledError('Session changed')
        get().auth.setPermissions(menuService.getPermissions())
        return menus
      },
      refreshToken: initRefreshToken,
      setRefreshToken: (refreshToken) =>
        set((state) => {
          persistRefreshToken(refreshToken)
          return { ...state, auth: { ...state.auth, refreshToken } }
        }),
      permissions: null,
      setPermissions: (permissions) =>
        set((state) => ({ ...state, auth: { ...state.auth, permissions } })),
      reset: () => {
        session.reset()
        menuService.clearCache()
        removeAllTokens()
        set((state) => ({
          auth: {
            ...state.auth,
            user: null,
            profile: null,
            isLoadingProfile: false,
            accessToken: '',
            refreshToken: '',
            permissions: null,
          },
        }))
      },
      clearSidebarCache: () => {
        menuService.clearCache()
        get().auth.setPermissions(null)
      },
    },
  }
})

registerAuthHandlers({
  reset: () => useAuthStore.getState().auth.reset(),
  setTokens: (accessToken, refreshToken) => {
    const { auth } = useAuthStore.getState()
    auth.setAccessToken(accessToken)
    if (refreshToken) auth.setRefreshToken(refreshToken)
  },
})
