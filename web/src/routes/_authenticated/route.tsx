import { createFileRoute, redirect } from '@tanstack/react-router'
import { menuService } from '@/services/menu-service'
import { useAuthStore } from '@/stores/auth-store'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'

// Routes that are always accessible for authenticated users (not menu-managed)
const ALWAYS_ALLOWED_PATHS = ['/', '/settings', '/errors']

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

    // Fetch profile if not already loaded
    if (!auth.profile && auth.accessToken) {
      try {
        await auth.fetchProfile()
      } catch (error) {
        console.error('Failed to fetch profile on route load:', error)
      }
    }

    // Load menu data (cached after first load) and check route authorization
    await menuService.loadMenuData()

    const pathname = location.pathname

    // Skip check for always-allowed non-menu routes
    const isAlwaysAllowed = ALWAYS_ALLOWED_PATHS.some(
      (p) => pathname === p || pathname.startsWith(p + '/')
    )

    if (!isAlwaysAllowed && !menuService.isPathAllowed(pathname)) {
      throw redirect({ to: '/403' })
    }
  },
  component: AuthenticatedLayout,
})
