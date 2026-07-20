import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useRef,
} from 'react'
import GoCaptcha from 'go-captcha-react'
import { Skeleton } from '@/components/ui/skeleton'
import {
    type SlideCaptchaHandle, type SlideCaptchaResult,
    useSlideCaptcha,
} from '../hooks/use-slide-captcha'

interface SlideCaptchaProps {
  onVerified: (result: SlideCaptchaResult) => void
  submitting?: boolean
}

export const SlideCaptcha = forwardRef<SlideCaptchaHandle, SlideCaptchaProps>(
  function SlideCaptcha({ onVerified, submitting }, ref) {
    const { challenge, verified, setVerified, fetchChallenge } = useSlideCaptcha()
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
    return (
      <div className='flex flex-col gap-3'>
        {challenge ? (
          <div className='flex justify-center'>
            <GoCaptcha.Slide
              ref={(r: unknown) => {
                slideRef.current = r as typeof slideRef.current
              }}
              data={{
                image: challenge.master_image,
                thumb: challenge.tile_image,
                thumbX: challenge.tile_x,
                thumbY: challenge.tile_y,
                thumbWidth: challenge.tile_width,
                thumbHeight: challenge.tile_height,
              }}
              config={{
                width: challenge.master_width,
                height: challenge.master_height,
                showTheme: false,
              }}
              events={{
                confirm: handleConfirm,
                refresh: fetchChallenge,
              }}
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
