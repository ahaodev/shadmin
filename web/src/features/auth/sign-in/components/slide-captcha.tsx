import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import { getSlideCaptcha, type SlideCaptchaChallenge } from '@/services/authApi'
import GoCaptcha from 'go-captcha-react'
import { toast } from 'sonner'
import { Skeleton } from '@/components/ui/skeleton'

export interface SlideCaptchaResult {
  captcha_id: string
  captcha_x: number
  captcha_y: number
}

export interface SlideCaptchaHandle {
  reset: () => void
  refresh: () => Promise<void>
}

interface SlideCaptchaProps {
  onVerified: (result: SlideCaptchaResult) => void
  submitting?: boolean
}

export const SlideCaptcha = forwardRef<SlideCaptchaHandle, SlideCaptchaProps>(
  function SlideCaptcha({ onVerified, submitting }, ref) {
    const [challenge, setChallenge] = useState<SlideCaptchaChallenge | null>(
      null
    )
    const [remaining, setRemaining] = useState(0)
    const [verified, setVerified] = useState(false)
    const slideRef = useRef<{
      reset: () => void
      clear: () => void
      refresh: () => void
    } | null>(null)
    const lastIdRef = useRef<string>('')
    const mountedRef = useRef(true)

    useEffect(() => {
      mountedRef.current = true
      return () => {
        mountedRef.current = false
      }
    }, [])

    const fetchChallenge = useCallback(async () => {
      try {
        const resp = await getSlideCaptcha(lastIdRef.current || undefined)
        if (!mountedRef.current) return
        if (resp?.code !== 0 || !resp.data) {
          toast.error(resp?.msg || '获取验证码失败')
          return
        }
        lastIdRef.current = resp.data.captcha_id
        setChallenge(resp.data)
        setRemaining(resp.data.expires_in)
        setVerified(false)
        slideRef.current?.reset()
      } catch (err) {
        if (!mountedRef.current) return
        // eslint-disable-next-line no-console
        console.error('Failed to load slide captcha', err)
        toast.error('获取验证码失败，请重试')
      }
    }, [])

    useEffect(() => {
      fetchChallenge()
    }, [fetchChallenge])

    useEffect(() => {
      if (!challenge || verified) return
      if (remaining <= 0) {
        fetchChallenge()
        return
      }
      const t = window.setTimeout(() => setRemaining((r) => r - 1), 1000)
      return () => window.clearTimeout(t)
    }, [remaining, challenge, verified, fetchChallenge])

    useImperativeHandle(
      ref,
      () => ({
        reset: () => {
          setVerified(false)
          slideRef.current?.reset()
        },
        refresh: fetchChallenge,
      }),
      [fetchChallenge]
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
      [challenge, verified, submitting, onVerified]
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
