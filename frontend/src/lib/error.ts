import { AxiosError } from 'axios'

export function getErrorMessage(
  error: unknown,
  fallback: string = '操作失败'
): string {
  if (error instanceof AxiosError) {
    return (
      ((error.response?.data as Record<string, unknown>)?.msg as string) ??
      fallback
    )
  }
  if (error instanceof Error) {
    return error.message
  }
  return fallback
}
