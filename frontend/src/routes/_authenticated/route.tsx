import { createFileRoute, redirect, useRouter } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { menuService } from '@/lib/menu-service'
import { Button } from '@/components/ui/button'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'

// Authenticated routes outside menu authorization.
const ALWAYS_ALLOWED_PATHS = ['/', '/settings', '/errors', '/auth/device']

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    const { auth } = useAuthStore.getState()

    if (!auth.accessToken) {
      throw redirect({
        to: '/sign-in',
        search: {
          redirect: location.href,
        },
      })
    }

    if (!auth.profile && auth.accessToken) {
      try {
        await auth.fetchProfile()
      } catch (error) {
        console.error('Failed to fetch profile on route load:', error)
      }
    }

    if (!useAuthStore.getState().auth.accessToken) {
      throw redirect({ to: '/sign-in', search: { redirect: location.href } })
    }

    await auth.loadMenus()

    const pathname = location.pathname

    const isAlwaysAllowed = ALWAYS_ALLOWED_PATHS.some(
      (p) => pathname === p || pathname.startsWith(p + '/')
    )

    if (!isAlwaysAllowed && !menuService.isPathAllowed(pathname)) {
      throw redirect({ to: '/403' })
    }
  },
  errorComponent: function PermissionLoadError() {
    const router = useRouter()
    return (
      <div className='flex min-h-svh flex-col items-center justify-center gap-4'>
        <p>无法加载账户权限，请重试。</p>
        <Button onClick={() => void router.invalidate()}>重试</Button>
      </div>
    )
  },
  component: AuthenticatedLayout,
})
