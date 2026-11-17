import z from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { LoginLogs } from '@/features/system/login-logs'

const loginLogsSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(10),
  email: z.string().optional().catch(''),
  status: z
    .array(z.union([z.literal('success'), z.literal('failed')]))
    .optional()
    .catch([]),
})

export const Route = createFileRoute('/_authenticated/system/login-logs')({
  validateSearch: loginLogsSearchSchema,
  component: LoginLogs,
})
