import { useMemo, useState } from 'react'
import { AxiosError } from 'axios'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useDeviceActivate } from '../hooks/use-device-activate'

function formatUserCode(value: string) {
  const normalized = value
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .slice(0, 8)
  if (normalized.length <= 4) return normalized
  return `${normalized.slice(0, 4)}-${normalized.slice(4)}`
}

function getErrorMessage(error: unknown) {
  if (error instanceof AxiosError) {
    return error.response?.data?.msg || error.message
  }
  if (error instanceof Error) return error.message
  return '设备授权失败，请重试'
}

export function DeviceActivateForm() {
  const [userCode, setUserCode] = useState('')
  const [authorized, setAuthorized] = useState(false)
  const activateMutation = useDeviceActivate()

  const canSubmit = useMemo(
    () => userCode.replace('-', '').length === 8 && !activateMutation.isPending,
    [activateMutation.isPending, userCode]
  )

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canSubmit) return

    try {
      const resp = await activateMutation.mutateAsync({ user_code: userCode })
      if (resp.code !== 0) {
        toast.error(resp.msg || '设备授权失败')
        return
      }
      setAuthorized(true)
      toast.success('设备授权成功，请回到 CLI 继续')
    } catch (error) {
      toast.error(getErrorMessage(error))
    }
  }

  return (
    <form onSubmit={handleSubmit} className='space-y-4'>
      <div className='space-y-2'>
        <label className='text-sm font-medium' htmlFor='device-user-code'>
          授权码
        </label>
        <Input
          id='device-user-code'
          placeholder='XXXX-XXXX'
          value={userCode}
          onChange={(event) => setUserCode(formatUserCode(event.target.value))}
          disabled={authorized || activateMutation.isPending}
          autoComplete='one-time-code'
          className='text-center font-mono text-lg tracking-[0.35em]'
        />
        <p className='text-muted-foreground text-sm'>
          输入终端显示的 8 位授权码。
        </p>
      </div>

      {authorized ? (
        <div className='rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-300'>
          授权成功。请返回设备，等待登录完成。
        </div>
      ) : null}

      <Button
        type='submit'
        className='w-full'
        disabled={!canSubmit || authorized}
      >
        {activateMutation.isPending ? (
          <Loader2 className='animate-spin' />
        ) : null}
        确认授权
      </Button>
    </form>
  )
}
