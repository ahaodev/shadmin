import { useEffect, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { updateProfile } from '@/services'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

const profileFormSchema = z.object({
  username: z
    .string('请输入您的用户名。')
    .min(2, '用户名最少必须包含2个字符。')
    .max(30, '用户名不能超过30个字符。'),
  email: z.string().email('请输入有效的邮箱地址。'),
  bio: z.string().max(160).min(4),
})

type ProfileFormValues = z.infer<typeof profileFormSchema>

const defaultValues: Partial<ProfileFormValues> = {
  username: '',
  email: 'admin@example.com',
  bio: '',
}

export function ProfileForm() {
  const { profile, fetchProfile, isLoadingProfile, setProfile } = useAuthStore(
    (state) => state.auth
  )
  const [isPending, setIsPending] = useState(false)

  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileFormSchema),
    defaultValues,
    mode: 'onChange',
  })

  // Fetch profile data and populate form when component loads
  useEffect(() => {
    fetchProfile()
  }, [fetchProfile])

  // Update form values when profile data is available
  useEffect(() => {
    if (profile) {
      form.reset({
        username: profile.username || '',
        email: profile.email || '',
        bio: profile.bio || '',
      })
    }
  }, [profile, form])

  async function onSubmit(data: ProfileFormValues) {
    try {
      setIsPending(true)
      await updateProfile({ bio: data.bio })
      // Refresh profile in store to reflect changes
      const updatedProfile = profile ? { ...profile, bio: data.bio } : profile
      setProfile(updatedProfile)
      toast.success('个人资料更新成功！')
    } catch {
      toast.error('更新失败，请重试。')
    } finally {
      setIsPending(false)
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        <div className='flex items-center gap-4'>
          <Avatar className='h-20 w-20 rounded-full'>
            <AvatarImage
              src={profile?.avatar || undefined}
              alt={profile?.username}
            />
            <AvatarFallback className='rounded-full text-2xl'>
              {profile?.username
                ? profile.username.charAt(0).toUpperCase()
                : 'A'}
            </AvatarFallback>
          </Avatar>
          <div className='space-y-1'>
            <p className='text-sm font-medium'>头像</p>
            <p className='text-muted-foreground text-xs'>
              {profile?.avatar
                ? '当前显示账号头像。'
                : '暂未设置头像，绑定第三方账号后将自动显示。'}
            </p>
          </div>
        </div>
        <FormField
          control={form.control}
          name='username'
          render={({ field }) => (
            <FormItem>
              <FormLabel>用户名</FormLabel>
              <FormControl>
                <Input placeholder='请输入用户名' disabled {...field} />
              </FormControl>
              <FormDescription>这是您的公开显示名称。</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>邮箱</FormLabel>
              <FormControl>
                <Input
                  type='email'
                  placeholder='请输入邮箱地址'
                  disabled
                  {...field}
                />
              </FormControl>
              <FormDescription>当前账户绑定的邮箱地址。</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='bio'
          render={({ field }) => (
            <FormItem>
              <FormLabel>个人简介</FormLabel>
              <FormControl>
                <Textarea
                  placeholder='简单介绍一下您自己'
                  className='resize-none'
                  {...field}
                />
              </FormControl>
              <FormDescription>个人简介最多160个字符。</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button type='submit' disabled={isLoadingProfile || isPending}>
          {isPending
            ? '保存中...'
            : isLoadingProfile
              ? '加载中...'
              : '更新个人资料'}
        </Button>
      </form>
    </Form>
  )
}
