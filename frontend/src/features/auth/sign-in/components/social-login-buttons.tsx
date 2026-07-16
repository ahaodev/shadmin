import { useEffect, useState } from 'react'
import {
  getSocialLoginHref,
  getSocialProviders,
  type SocialProvider,
} from '@/services/authApi'
import { Loader2 } from 'lucide-react'
import { IconGithub } from '@/assets/brand-icons/icon-github'
import { IconGmail } from '@/assets/brand-icons/icon-gmail'
import { Button } from '@/components/ui/button'

const PROVIDER_ICON_CLASS_NAME = 'size-4 shrink-0'

function providerIcon(provider: SocialProvider['provider']) {
  switch (provider) {
    case 'github':
      return <IconGithub className={PROVIDER_ICON_CLASS_NAME} />
    case 'google':
      return <IconGmail className={PROVIDER_ICON_CLASS_NAME} />
    default:
      return null
  }
}

// SocialLoginButtons 拉取后端已启用的第三方登录 provider 列表，
// 仅在后端启用至少一个 provider 时显示对应按钮；未启用时整组不显示。
export function SocialLoginButtons() {
  const [providers, setProviders] = useState<SocialProvider[] | null>(null)
  const [isError, setIsError] = useState(false)

  useEffect(() => {
    let cancelled = false

    void (async () => {
      try {
        const response = await getSocialProviders()
        if (cancelled) return

        if (response?.code === 0 && Array.isArray(response.data)) {
          setProviders(response.data)
          return
        }

        setIsError(true)
      } catch {
        if (cancelled) return
        // 接口不可用或未配置：静默隐藏整组按钮
        setIsError(true)
      }
    })()

    return () => {
      cancelled = true
    }
  }, [])

  if (isError) return null

  if (providers === null) {
    // 拉取中：占位避免布局抖动
    return (
      <div className='flex items-center justify-center'>
        <Loader2 className='text-muted-foreground size-4 animate-spin' />
      </div>
    )
  }

  if (providers.length === 0) return null

  return (
    <div className='mt-4 space-y-3'>
      <div className='flex items-center gap-3'>
        <div className='bg-border h-px flex-1' />
        <span className='text-muted-foreground text-sm'>OR</span>
        <div className='bg-border h-px flex-1' />
      </div>
      <div className='flex flex-wrap justify-center gap-2'>
        {providers.map((provider) => (
          <Button
            key={provider.provider}
            type='button'
            variant='outline'
            className='flex min-w-35 flex-1 justify-center gap-2'
            // 直接跳转到后端 OAuth 入口；后端负责重定向到 provider 授权页
            asChild
          >
            <a
              href={getSocialLoginHref(provider.provider)}
              className='flex w-full items-center justify-center gap-2'
              aria-label={`Continue with ${provider.name}`}
            >
              <span className='flex items-center gap-2'>
                {providerIcon(provider.provider)}
                <span>{provider.name}</span>
              </span>
            </a>
          </Button>
        ))}
      </div>
    </div>
  )
}
