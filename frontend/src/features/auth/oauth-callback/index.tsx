import { useEffect } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { exchangeUserIdentityCode } from '@/services/authApi'
import { getProfile } from '@/services/profileApi'
import { useAuthStore } from '@/stores/auth-store'
import { authUserFromJwt } from '@/lib/jwt'

const pendingIdentityCodeExchanges = new Map<
  string,
  Promise<Awaited<ReturnType<typeof exchangeUserIdentityCode>>>
>()

async function exchangeUserIdentityCodeOnce(code: string) {
  const existing = pendingIdentityCodeExchanges.get(code)
  if (existing) {
    return existing
  }

  const pending = exchangeUserIdentityCode(code)
  pendingIdentityCodeExchanges.set(code, pending)

  try {
    return await pending
  } finally {
    pendingIdentityCodeExchanges.delete(code)
  }
}

export function OAuthCallback() {
  const navigate = useNavigate()
  const { code, error } = useSearch({ from: '/(auth)/oauth-callback' })

  useEffect(() => {
    if (error) {
      navigate({ to: '/sign-in', search: { error: 'oauth' }, replace: true })
      return
    }

    if (!code) {
      navigate({ to: '/sign-in', search: { error: 'oauth' }, replace: true })
      return
    }

    void (async () => {
      try {
        const response = await exchangeUserIdentityCodeOnce(code)
        if (response.code !== 0 || !response.data) {
          navigate({
            to: '/sign-in',
            search: { error: 'oauth' },
            replace: true,
          })
          return
        }

        const { accessToken, refreshToken } = response.data

        const {
          setUser,
          setAccessToken,
          setRefreshToken,
          setProviderAvatar,
          clearSidebarCache,
          fetchProfile,
        } = useAuthStore.getState().auth

        const user = authUserFromJwt(accessToken)

        if (user) {
          setUser({
            accountNo: user.accountNo,
            email: user.email,
            role: user.role,
            exp: user.exp,
          })
        }

        setAccessToken(accessToken)
        if (refreshToken) {
          setRefreshToken(refreshToken)
        }
        setProviderAvatar(response.data.providerAvatarUrl ?? null)
        clearSidebarCache()

        try {
          await getProfile()
          await fetchProfile()
        } catch (e) {
          console.warn('Failed to fetch profile after identity login:', e)
        }

        try {
          const { menuService } = await import('@/services/menu-service')
          await menuService.reloadMenuData()
        } catch (e) {
          console.warn('Failed to reload menu data after identity login:', e)
        }

        navigate({ to: '/', replace: true })
      } catch (caughtError) {
        console.warn('Failed to exchange identity login code:', caughtError)
        navigate({ to: '/sign-in', search: { error: 'oauth' }, replace: true })
      }
    })()
  }, [code, error, navigate])

  return (
    <div className='flex min-h-svh w-full items-center justify-center'>
      <div className='text-muted-foreground flex items-center gap-2'>
        <Loader2 className='animate-spin' />
        <span>正在完成第三方登录...</span>
      </div>
    </div>
  )
}
