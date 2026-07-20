import { useCallback, useEffect, useRef, useState } from 'react'
import { getSlideCaptcha, type SlideCaptchaChallenge } from '@/services/authApi'
import { toast } from 'sonner'

export interface SlideCaptchaResult {
  captcha_id: string
  captcha_x: number
  captcha_y: number
}

export interface SlideCaptchaHandle {
  reset: () => void
  refresh: () => Promise<void>
}

export function useSlideCaptcha() {
  const [challenge, setChallenge] = useState<SlideCaptchaChallenge | null>(null)
  const [remaining, setRemaining] = useState(0)
  const [verified, setVerified] = useState(false)
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
    } catch (err) {
      if (!mountedRef.current) return
      // eslint-disable-next-line no-console
      console.error('Failed to load slide captcha', err)
      toast.error('获取验证码失败，请重试')
    }
  }, [setVerified])

  useEffect(() => {
    void fetchChallenge()
  }, [fetchChallenge])

  useEffect(() => {
    if (!challenge || verified) return
    if (remaining <= 0) {
      void fetchChallenge()
      return
    }

    const t = window.setTimeout(() => setRemaining((r) => r - 1), 1000)
    return () => window.clearTimeout(t)
  }, [remaining, challenge, verified, fetchChallenge])

  return {
    challenge,
    remaining,
    verified,
    setVerified,
    fetchChallenge,
  }
}
