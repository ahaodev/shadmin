import { useEffect } from 'react'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { getProfile } from '@/services/profileApi'
import { exchangeSocialCode } from '@/services/authApi'
import { Loader2 } from 'lucide-react'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

/**
 * 第三方登录回调落地页。
 *
 * 后端在 OAuth 成功后会重定向到：
 *   /oauth/callback?code=...
 * 失败则重定向到 /sign-in?error=oauth（或 disabled）。
 *
 * 本路由职责：
 *  1. 用一次性 code 兑换 JWT；
 *  2. 将令牌写入 auth-store；
 *  3. 拉取用户 profile 与菜单，然后导航到首页。
 */
export const Route = createFileRoute('/(auth)/oauth-callback')({
  component: OAuthCallbackComponent,
})

function decodeJwt(token: string): Record<string, unknown> | null {
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

function OAuthCallbackComponent() {
  const navigate = useNavigate()

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const code = params.get('code')

    if (!code) {
      navigate({ to: '/sign-in', search: { error: 'oauth' }, replace: true })
      return
    }

    void (async () => {
      try {
        const response = await exchangeSocialCode(code)
        if (response.code !== 0 || !response.data) {
          navigate({ to: '/sign-in', search: { error: 'oauth' }, replace: true })
          return
        }

        const { accessToken, refreshToken } = response.data
        const tokenPayload = decodeJwt(accessToken) || {}

        // 直接从 store 读取稳定 action 引用，避免把 auth 快照放进依赖数组
        // 导致 setUser/setAccessToken 触发 useEffect 重跑。
        const {
          setUser,
          setAccessToken,
          setRefreshToken,
          clearSidebarCache,
          fetchProfile,
        } = useAuthStore.getState().auth

        const user: AuthUser = {
          accountNo: String(
            tokenPayload.name ?? tokenPayload.username ?? tokenPayload.id ?? ''
          ),
          email: '',
          role: [],
          exp:
            typeof tokenPayload.exp === 'number'
              ? tokenPayload.exp
              : Date.now() + 24 * 60 * 60,
        }

        setUser(user)
        setAccessToken(accessToken)
        if (refreshToken) {
          setRefreshToken(refreshToken)
        }
        clearSidebarCache()

        try {
          // 预取 profile，确保 /api/v1/profile 走带新令牌的请求
          await getProfile()
          await fetchProfile()
        } catch (e) {
          console.warn('Failed to fetch profile after social login:', e)
        }

        try {
          const { menuService } = await import('@/services/menu-service')
          await menuService.reloadMenuData()
        } catch (e) {
          console.warn('Failed to reload menu data after social login:', e)
        }

        navigate({ to: '/', replace: true })
      } catch (error) {
        console.warn('Failed to exchange social login code:', error)
        navigate({ to: '/sign-in', search: { error: 'oauth' }, replace: true })
      }
    })()
  }, [navigate])

  return (
    <div className='flex min-h-svh w-full items-center justify-center'>
      <div className='text-muted-foreground flex items-center gap-2'>
        <Loader2 className='animate-spin' />
        <span>正在完成第三方登录...</span>
      </div>
    </div>
  )
}
