import { useEffect } from 'react'
import { useSearch } from '@tanstack/react-router'
import { toast } from 'sonner'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
import { SocialLoginButtons } from './components/social-login-buttons'
import { UserAuthForm } from './components/user-auth-form'

const errorMessages: Record<string, string> = {
  oauth: '第三方登录失败，请重试',
  disabled: '该账户已被停用，请联系管理员',
}

export function SignIn() {
  const { redirect, error } = useSearch({ from: '/(auth)/sign-in' })

  useEffect(() => {
    if (error && errorMessages[error]) {
      toast.error(errorMessages[error])
    }
  }, [error])

  return (
    <AuthLayout>
      <Card className='gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>登录</CardTitle>
          <CardDescription>
            输入您的用户名和密码 <br />
          </CardDescription>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={redirect} />
          <SocialLoginButtons />
        </CardContent>
        <CardFooter className='flex justify-center'>
          <p className='text-muted-foreground px-8 text-center text-sm'>
            登录后使用
          </p>
        </CardFooter>
      </Card>
    </AuthLayout>
  )
}
