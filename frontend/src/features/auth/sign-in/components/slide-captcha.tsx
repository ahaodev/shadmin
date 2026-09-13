import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useMemo,
  useRef,
} from 'react'
import GoCaptcha from 'go-captcha-react'
import { Skeleton } from '@/components/ui/skeleton'
import {
  type SlideCaptchaHandle,
  type SlideCaptchaResult,
  useSlideCaptcha,
} from '../hooks/use-slide-captcha'

interface SlideCaptchaProps {
  onVerified: (result: SlideCaptchaResult) => void
  submitting?: boolean
}

export const SlideCaptcha = forwardRef<SlideCaptchaHandle, SlideCaptchaProps>(
  function SlideCaptcha({ onVerified, submitting }, ref) {
    const { challenge, verified, setVerified, fetchChallenge } =
      useSlideCaptcha()
    const slideRef = useRef<{
      reset: () => void
      clear: () => void
      refresh: () => void
    } | null>(null)

    useImperativeHandle(
      ref,
      () => ({
        reset: () => {
          setVerified(false)
          slideRef.current?.reset()
        },
        refresh: async () => {
          await fetchChallenge()
          slideRef.current?.reset()
        },
      }),
      [fetchChallenge, setVerified]
    )

    const handleConfirm = useCallback(
      (point: { x: number; y: number }, reset: () => void) => {
        if (!challenge || verified || submitting) {
          reset()
          return
        }
        setVerified(true)
        onVerified({
          captcha_id: challenge.captcha_id,
          captcha_x: Math.round(point.x),
          captcha_y: Math.round(point.y),
        })
      },
      [challenge, verified, submitting, onVerified, setVerified]
    )

    // 倒计时每秒触发一次重渲染，这里固定 data/config/events 的引用，
    // 避免 go-captcha-react 每秒都执行一次内部 state 更新。
    const captchaData = useMemo(
      () => ({
        image: challenge?.master_image ?? '',
        thumb: challenge?.tile_image ?? '',
        thumbX: challenge?.tile_x ?? 0,
        thumbY: challenge?.tile_y ?? 0,
        thumbWidth: challenge?.tile_width ?? 0,
        thumbHeight: challenge?.tile_height ?? 0,
      }),
      [challenge]
    )
    const captchaConfig = useMemo(
      () => ({
        width: challenge?.master_width ?? 0,
        height: challenge?.master_height ?? 0,
        showTheme: false,
      }),
      [challenge]
    )
    const captchaEvents = useMemo(
      () => ({ confirm: handleConfirm, refresh: fetchChallenge }),
      [handleConfirm, fetchChallenge]
    )

    return (
      <div className='flex flex-col gap-3'>
        {challenge ? (
          <div className='flex justify-center'>
            {/* go-captcha-react 在 data 变化时会保留上一份挑战的内部引用（实测每次刷新残留
                约 80KB 图片数据、不会被 GC 回收），以 captcha_id 作 key 强制重建实例。 */}
            <GoCaptcha.Slide
              key={challenge.captcha_id}
              ref={(r: unknown) => {
                slideRef.current = r as typeof slideRef.current
              }}
              data={captchaData}
              config={captchaConfig}
              events={captchaEvents}
            />
          </div>
        ) : (
          <div className='flex flex-col gap-2'>
            <Skeleton className='h-40 w-full' />
            <Skeleton className='h-10 w-full' />
          </div>
        )}
      </div>
    )
  }
)
