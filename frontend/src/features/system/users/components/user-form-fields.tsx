import { type UseFormReturn } from 'react-hook-form'
import { useQuery } from '@tanstack/react-query'
import { getDepartmentTree } from '@/services/departmentApi'
import type { Department } from '@/types/department'
import type { RoleInfo } from '@/types/role'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { MultiSelectDropdown } from '@/components/multi-select-dropdown'
import { PasswordInput } from '@/components/password-input'
import { SelectDropdown } from '@/components/select-dropdown'
import { statusOptions, type UserFormData } from '../lib/user-form-schema'

interface UserFormFieldsProps {
  form: UseFormReturn<UserFormData>
  isEdit: boolean
  isPasswordTouched: boolean
  allRoles?: RoleInfo[]
}

function flattenDepartmentsForSelect(
  departments: Department[] | null | undefined,
  level = 0
): { id: string; name: string; level: number }[] {
  if (!Array.isArray(departments)) {
    return []
  }
  const result: { id: string; name: string; level: number }[] = []
  for (const dept of departments) {
    if (!dept) continue
    result.push({ id: dept.id, name: dept.name, level })
    if (dept.children) {
      result.push(...flattenDepartmentsForSelect(dept.children, level + 1))
    }
  }
  return result
}

export function UserFormFields({
  form,
  isPasswordTouched,
  allRoles = [],
}: UserFormFieldsProps) {
  const { data: departmentTree = [] } = useQuery({
    queryKey: ['departments'],
    queryFn: getDepartmentTree,
    staleTime: 5 * 60 * 1000,
  })
  const departmentOptions = flattenDepartmentsForSelect(departmentTree)

  return (
    <>
      <FormField
        control={form.control}
        name='username'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>用户名</FormLabel>
            <FormControl>
              <Input placeholder='用户名' className='col-span-4' {...field} />
            </FormControl>
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='email'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>邮箱</FormLabel>
            <FormControl>
              <Input
                placeholder='例如：john.doe@gmail.com'
                className='col-span-4'
                {...field}
              />
            </FormControl>
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='phone'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>手机号码</FormLabel>
            <FormControl>
              <Input
                placeholder='+86 13812345678'
                className='col-span-4'
                {...field}
              />
            </FormControl>
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='status'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>状态</FormLabel>
            <SelectDropdown
              defaultValue={field.value}
              onValueChange={field.onChange}
              placeholder='选择状态'
              className='col-span-4'
              items={[...statusOptions]}
            />
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='password'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>密码</FormLabel>
            <FormControl>
              <PasswordInput
                placeholder='例如：S3cur3P@ssw0rd'
                className='col-span-4'
                {...field}
              />
            </FormControl>
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='confirmPassword'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>确认密码</FormLabel>
            <FormControl>
              <PasswordInput
                disabled={!isPasswordTouched}
                placeholder='例如：S3cur3P@ssw0rd'
                className='col-span-4'
                {...field}
              />
            </FormControl>
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='roles'
        render={({ field }) => {
          const selectedRoles = field.value || []

          const handleRoleChange = (roleIds: string[]) => {
            const roleNames = roleIds.map((id) => {
              const role = allRoles.find((r) => r.id === id)
              return role?.name || id
            })
            field.onChange(roleNames)
          }

          const selectedRoleIds = selectedRoles.map((roleName: string) => {
            const role = allRoles.find((r) => r.name === roleName)
            return role?.id || roleName
          })

          const roleItems = allRoles.map((role) => ({
            label: role.name,
            value: role.id,
          }))

          return (
            <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
              <FormLabel className='col-span-2 text-end'>角色</FormLabel>
              <FormControl>
                <MultiSelectDropdown
                  value={selectedRoleIds}
                  onValueChange={handleRoleChange}
                  items={roleItems}
                  placeholder='选择角色'
                  className='col-span-4'
                />
              </FormControl>
              <FormMessage className='col-span-4 col-start-3' />
            </FormItem>
          )
        }}
      />

      <FormField
        control={form.control}
        name='department_id'
        render={({ field }) => (
          <FormItem className='grid grid-cols-6 items-center space-y-0 gap-x-4 gap-y-1'>
            <FormLabel className='col-span-2 text-end'>部门</FormLabel>
            <Select
              onValueChange={(val) =>
                field.onChange(val === '__none__' ? '' : val)
              }
              value={field.value || '__none__'}
            >
              <FormControl>
                <SelectTrigger className='col-span-4'>
                  <SelectValue placeholder='选择部门' />
                </SelectTrigger>
              </FormControl>
              <SelectContent>
                <SelectItem value='__none__'>无</SelectItem>
                {departmentOptions.map((dept) => (
                  <SelectItem key={dept.id} value={dept.id}>
                    {'　'.repeat(dept.level)}
                    {dept.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <FormMessage className='col-span-4 col-start-3' />
          </FormItem>
        )}
      />
    </>
  )
}
