import { useEffect, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { acceptUserInvitation } from '@/services/userApi'
import { getErrorMessage } from '@/lib/error'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import { AuthLayout } from '../auth-layout'

const formSchema = z
  .object({
    username: z
      .string()
      .trim()
      .min(1, '请输入用户名')
      .max(32, '用户名最多32个字符'),
    password: z
      .string()
      .min(8, '密码至少8个字符')
      .refine((value) => new TextEncoder().encode(value).length <= 72, {
        message: '密码最多72字节',
      }),
    confirmPassword: z.string().min(1, '请确认密码'),
  })
  .refine((values) => values.password === values.confirmPassword, {
    message: '两次输入的密码不一致',
    path: ['confirmPassword'],
  })

type FormValues = z.infer<typeof formSchema>

export function AcceptInvitation() {
  const [token] = useState(() => {
    if (typeof window === 'undefined') return ''
    return new URLSearchParams(window.location.hash.slice(1)).get('token') || ''
  })
  const [accepted, setAccepted] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    if (window.location.hash) {
      window.history.replaceState(
        null,
        '',
        `${window.location.pathname}${window.location.search}`
      )
    }
  }, [])

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { username: '', password: '', confirmPassword: '' },
  })

  const onSubmit = async (values: FormValues) => {
    setErrorMessage('')
    try {
      await acceptUserInvitation({
        token,
        username: values.username,
        password: values.password,
      })
      setAccepted(true)
      form.reset()
    } catch (error) {
      setErrorMessage(
        getErrorMessage(error, '接受邀请失败，请确认链接有效后重试。')
      )
    }
  }

  return (
    <AuthLayout>
      <Card className='gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>接受邀请</CardTitle>
          <CardDescription>
            {accepted
              ? '账户已启用，现在可以登录。'
              : '设置用户名和密码以完成账户创建。'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {accepted ? (
            <Button asChild className='w-full'>
              <Link to='/sign-in'>前往登录</Link>
            </Button>
          ) : !token ? (
            <div className='space-y-4'>
              <p className='text-destructive text-sm'>
                邀请链接无效或已过期，请联系邀请人重新发送。
              </p>
              <Button asChild variant='outline' className='w-full'>
                <Link to='/sign-in'>返回登录</Link>
              </Button>
            </div>
          ) : (
            <Form {...form}>
              <form
                onSubmit={form.handleSubmit(onSubmit)}
                className='grid gap-4'
              >
                <FormField
                  control={form.control}
                  name='username'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>用户名</FormLabel>
                      <FormControl>
                        <Input autoComplete='username' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='password'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>密码</FormLabel>
                      <FormControl>
                        <PasswordInput autoComplete='new-password' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='confirmPassword'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>确认密码</FormLabel>
                      <FormControl>
                        <PasswordInput autoComplete='new-password' {...field} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                {errorMessage && (
                  <p role='alert' className='text-destructive text-sm'>
                    {errorMessage}
                  </p>
                )}
                <Button type='submit' disabled={form.formState.isSubmitting}>
                  {form.formState.isSubmitting ? '处理中...' : '完成注册'}
                </Button>
              </form>
            </Form>
          )}
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
