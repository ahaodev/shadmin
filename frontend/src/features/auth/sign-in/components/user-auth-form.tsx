import { useCallback, useRef, useState } from 'react'
import { z } from 'zod'
import { AxiosError } from 'axios'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link, useNavigate } from '@tanstack/react-router'
import { login } from '@/services/authApi'
import { Loader2, LogIn } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { authUserFromJwt } from '@/lib/jwt'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import {
  SlideCaptcha,
} from './slide-captcha'
import {type SlideCaptchaHandle, type SlideCaptchaResult} from "@/features/auth/sign-in/hooks/use-slide-captcha.ts";

const formSchema = z.object({
  username: z.string().min(1, '请输入用户名'),
  password: z.string().min(1, '请输入密码'),
})

type FormValues = z.infer<typeof formSchema>

interface UserAuthFormProps extends React.HTMLAttributes<HTMLFormElement> {
  redirectTo?: string
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: UserAuthFormProps) {
  const [isLoading, setIsLoading] = useState(false)
  const [captchaOpen, setCaptchaOpen] = useState(false)
  const captchaRef = useRef<SlideCaptchaHandle>(null)
  const pendingValuesRef = useRef<FormValues | null>(null)
  const navigate = useNavigate()
  const { auth } = useAuthStore()

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  })

  function onSubmit(values: FormValues) {
    pendingValuesRef.current = values
    setCaptchaOpen(true)
  }

  const performLogin = useCallback(
    async (values: FormValues, captcha: SlideCaptchaResult) => {
      setIsLoading(true)

      const start =
        typeof performance !== 'undefined' ? performance.now() : Date.now()
      sessionStorage.setItem('loginStart', String(start))

      try {
        const resp = await login({
          username: values.username,
          password: values.password,
          captcha_id: captcha.captcha_id,
          captcha_x: captcha.captcha_x,
          captcha_y: captcha.captcha_y,
        })

        if (!resp || resp.code !== 0) {
          toast.error(resp?.msg || '登录失败')
          captchaRef.current?.refresh()
          return
        }

        const payload = resp.data

        const accessToken = payload?.accessToken
        if (!accessToken) {
          toast.error('未收到访问令牌，请重试')
          captchaRef.current?.refresh()
          return
        }

        const user = authUserFromJwt(accessToken)

        if (user) {
          auth.setUser({
            accountNo: user.accountNo,
            email: user.email,
            role: user.role,
            exp: user.exp,
          })
        }

        auth.setAccessToken(accessToken)

        const refreshTokenValue = payload?.refreshToken
        if (refreshTokenValue) {
          auth.setRefreshToken(refreshTokenValue)
        }

        auth.setProviderAvatar(null)
        auth.clearSidebarCache()

        try {
          await auth.fetchProfile()
        } catch (error) {
          // eslint-disable-next-line no-console
          console.warn('Failed to fetch profile after login:', error)
        }

        try {
          const { menuService } = await import('@/services/menu-service')
          await menuService.reloadMenuData()
        } catch (error) {
          // eslint-disable-next-line no-console
          console.warn('Failed to reload menu data after login:', error)
        }

        toast.success(
          `欢迎回来，${user?.accountNo ?? values.username ?? '用户'}！`
        )

        setCaptchaOpen(false)
        const targetPath = redirectTo || '/'
        navigate({ to: targetPath, replace: true })
      } catch (error: unknown) {
        // eslint-disable-next-line no-console
        console.error('Login error', error)
        let msg = '网络错误，请稍后重试'
        let isCredentialError = false
        if (error instanceof AxiosError) {
          msg = error.response?.data?.msg || error.message
          const status = error.response?.status
          // 401 = 账号密码错误, 423 = 账户锁定 — 无需刷新验证码，直接关闭弹窗
          isCredentialError = status === 401 || status === 423
        } else if (error instanceof Error) {
          msg = error.message
        }
        toast.error(msg)
        if (isCredentialError) {
          setCaptchaOpen(false)
        } else {
          captchaRef.current?.refresh()
        }
      } finally {
        setIsLoading(false)
      }
    },
    [auth, navigate, redirectTo]
  )

  const handleVerified = useCallback(
    (result: SlideCaptchaResult) => {
      const values = pendingValuesRef.current
      if (!values) {
        setCaptchaOpen(false)
        return
      }
      void performLogin(values, result)
    },
    [performLogin]
  )

  const handleOpenChange = useCallback(
    (next: boolean) => {
      if (isLoading) return
      setCaptchaOpen(next)
      if (!next) {
        pendingValuesRef.current = null
        captchaRef.current?.reset()
      }
    },
    [isLoading]
  )

  return (
    <>
      <Form {...form}>
        <form
          onSubmit={(event) => void form.handleSubmit(onSubmit)(event)}
          className={cn('grid gap-3', className)}
          {...props}
        >
          <FormField
            control={form.control}
            name='username'
            render={({ field }) => (
              <FormItem>
                <FormLabel>用户名</FormLabel>
                <FormControl>
                  <Input placeholder='用户名或邮箱或手机号' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='password'
            render={({ field }) => (
              <FormItem className='relative'>
                <FormLabel>密码</FormLabel>
                <FormControl>
                  <PasswordInput placeholder='密码' {...field} />
                </FormControl>
                <FormMessage />
                <Link
                  to='/forgot-password'
                  className='text-muted-foreground absolute end-0 -top-0.5 text-sm font-medium hover:opacity-75'
                >
                  忘记密码?
                </Link>
              </FormItem>
            )}
          />
          <Button className='mt-2' disabled={isLoading}>
            {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
            登入
          </Button>
        </form>
      </Form>

      <Dialog open={captchaOpen} onOpenChange={handleOpenChange}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>安全验证</DialogTitle>
          </DialogHeader>
          {captchaOpen ? (
            <SlideCaptcha
              ref={captchaRef}
              onVerified={handleVerified}
              submitting={isLoading}
            />
          ) : null}
        </DialogContent>
      </Dialog>
    </>
  )
}
