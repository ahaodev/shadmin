import { createFileRoute } from '@tanstack/react-router'
import { AcceptInvitation } from '@/features/auth/accept-invitation'

export const Route = createFileRoute('/(auth)/accept-invitation')({
  component: AcceptInvitation,
})
