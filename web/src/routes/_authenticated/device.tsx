import { createFileRoute } from '@tanstack/react-router'
import { DeviceActivate } from '@/features/auth/device-activate'

export const Route = createFileRoute('/_authenticated/device')({
  component: DeviceActivate,
})
