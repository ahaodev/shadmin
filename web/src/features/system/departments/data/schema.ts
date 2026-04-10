import { z } from 'zod'

export const departmentFormSchema = z.object({
  parent_id: z.string().optional(),
  name: z.string().min(1, '部门名称不能为空').max(100, '部门名称最多100个字符'),
  sequence: z.number().int().min(0, '排序值不能为负数'),
  leader: z.string().max(50, '负责人最多50个字符').optional(),
  phone: z.string().max(20, '电话最多20个字符').optional(),
  email: z.string().max(100, '邮箱最多100个字符').optional(),
  status: z.string().min(1, '状态为必填项'),
})

export type DepartmentFormData = z.infer<typeof departmentFormSchema>
