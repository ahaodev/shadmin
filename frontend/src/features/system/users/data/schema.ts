import { z } from 'zod'
import { isPasswordLengthValid } from '@/lib/password'

const passwordValidation = {
  hasValidLength: (password: string, isEdit: boolean) => {
    if (isEdit && !password) return true
    return isPasswordLengthValid(password)
  },
  hasLowercase: (password: string, isEdit: boolean) => {
    if (isEdit && !password) return true
    return /[a-z]/.test(password)
  },
  hasNumber: (password: string, isEdit: boolean) => {
    if (isEdit && !password) return true
    return /\d/.test(password)
  },
  isRequired: (password: string, isEdit: boolean) => {
    if (isEdit && !password) return true
    return password.length > 0
  },
  matchesConfirm: (
    password: string,
    confirmPassword: string,
    isEdit: boolean
  ) => {
    if (isEdit && !password) return true
    return password === confirmPassword
  },
}

export const userFormSchema = z
  .object({
    username: z.string().min(1, '用户名为必填项。'),
    email: z.email({
      error: (iss) => (iss.input === '' ? '邮箱为必填项。' : undefined),
    }),
    phone: z.string().optional(),
    password: z.string().transform((pwd) => pwd.trim()),
    confirmPassword: z.string().transform((pwd) => pwd.trim()),
    status: z.string().min(1, '状态为必填项。'),
    roles: z.array(z.string()),
    department_id: z.string().optional(),
    isEdit: z.boolean(),
  })
  .refine((data) => passwordValidation.isRequired(data.password, data.isEdit), {
    message: '密码为必填项。',
    path: ['password'],
  })
  .refine(
    ({ password, isEdit }) =>
      passwordValidation.hasValidLength(password, isEdit),
    {
      message: '密码至少 8 个字符，且不能超过 72 字节',
      path: ['password'],
    }
  )
  .refine(
    ({ password, isEdit }) => passwordValidation.hasLowercase(password, isEdit),
    {
      message: '密码必须包含至少一个小写字母。',
      path: ['password'],
    }
  )
  .refine(
    ({ password, isEdit }) => passwordValidation.hasNumber(password, isEdit),
    {
      message: '密码必须包含至少一个数字。',
      path: ['password'],
    }
  )
  .refine(
    ({ password, confirmPassword, isEdit }) =>
      passwordValidation.matchesConfirm(password, confirmPassword, isEdit),
    {
      message: '密码不匹配。',
      path: ['confirmPassword'],
    }
  )

export type UserFormData = z.infer<typeof userFormSchema>

export const statusOptions = [
  { label: '激活', value: 'active' },
  { label: '停用', value: 'inactive' },
  { label: '已邀请', value: 'invited' },
  { label: '暂停', value: 'suspended' },
] as const
