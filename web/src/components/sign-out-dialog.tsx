import { useNavigate } from '@tanstack/react-router'
import { logout } from '@/services/authApi'
import { useAuthStore } from '@/stores/auth-store'
import { ConfirmDialog } from '@/components/confirm-dialog'

interface SignOutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SignOutDialog({ open, onOpenChange }: SignOutDialogProps) {
  const navigate = useNavigate()
  const { auth } = useAuthStore()

  const handleSignOut = async () => {
    try {
      await logout()
    } catch (error) {
      console.warn(
        'Logout API failed, but continuing with local cleanup:',
        error
      )
    } finally {
      // auth.reset() clears all tokens, cookies, localStorage, menu cache, and zustand state
      auth.reset()

      navigate({
        to: '/sign-in',
        replace: true,
      })
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title='登出'
      desc='确定要登出吗？登出后需要重新登录才能访问您的账户'
      confirmText='登出'
      handleConfirm={handleSignOut}
      className='sm:max-w-sm'
    />
  )
}
