import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/error'

/** Refresh navigation and button permissions after authorization changes. */
export async function refreshPermissions() {
  const { auth } = useAuthStore.getState()
  if (!auth.accessToken) return
  try {
    await auth.loadMenus(true)
  } catch (error) {
    toast.error(getErrorMessage(error, '权限刷新失败，请刷新页面重试'))
  }
}
