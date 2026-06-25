import { useMutation } from '@tanstack/react-query'
import { activateDevice } from '@/services/authApi'

export function useDeviceActivate() {
  return useMutation({
    mutationFn: activateDevice,
  })
}
