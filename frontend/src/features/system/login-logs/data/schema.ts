import { z } from 'zod'

const loginLogStatusSchema = z.union([
  z.literal('success'),
  z.literal('failed'),
])

const loginLogSchema = z.object({
  id: z.string(),
  email: z.string().optional(),
  login_ip: z.string(),
  user_agent: z.string(),
  browser: z.string().optional(),
  os: z.string().optional(),
  device: z.string().optional(),
  status: loginLogStatusSchema,
  source: z.string().optional(),
  failure_reason: z.string().optional(),
  login_time: z.coerce.date(),
})
export type { LoginLog } from '@/types/login-log'

export const loginLogListSchema = z.array(loginLogSchema)

// 查询过滤器schema
export const loginLogFilterSchema = z.object({
  page: z.number().optional(),
  page_size: z.number().optional(),
  email: z.string().optional(),
  login_ip: z.string().optional(),
  status: loginLogStatusSchema.optional(),
  source: z.string().optional(),
  browser: z.string().optional(),
  os: z.string().optional(),
  start_time: z.string().optional(),
  end_time: z.string().optional(),
  sort_by: z.string().optional(),
  order: z.union([z.literal('asc'), z.literal('desc')]).optional(),
})
