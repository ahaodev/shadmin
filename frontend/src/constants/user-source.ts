
export const USER_SOURCE_LOCAL = 'shadmin'

const USER_SOURCE_LABELS: Record<string, string> = {
  [USER_SOURCE_LOCAL]: '本地',
  github: 'GitHub',
  google: 'Google',
}

export function userSourceLabel(source?: string): string {
  if (!source) return '未知'
  return USER_SOURCE_LABELS[source] ?? source
}
