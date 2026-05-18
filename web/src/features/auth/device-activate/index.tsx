import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { DeviceActivateForm } from './components/device-activate-form'

export function DeviceActivate() {
  return (
    <main className='bg-background flex min-h-svh flex-col items-center justify-center px-4 py-10'>
      <Card className='w-full max-w-120 shadow-lg'>
        <CardHeader className='space-y-2 text-center'>
          <CardTitle className='text-xl'>设备授权</CardTitle>
          <CardDescription>
            输入授权码，允许该设备访问你的账号。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <DeviceActivateForm />
        </CardContent>
      </Card>
      <p className='text-muted-foreground mt-6 max-w-[420px] text-center text-xs'>
        确认前请检查授权码来自你正在使用的 Shadmin CLI。
      </p>
    </main>
  )
}
