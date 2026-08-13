import { z } from 'zod'

export const roleSchema = z.object({
  name: z.string().min(1, '角色名称是必填的').max(100, '角色名称最多100个字符'),
  sequence: z.number().int().min(0, '序号必须是非负整数'),
  status: z.enum(['active', 'inactive']),
})

export type RoleFormData = z.infer<typeof roleSchema>
