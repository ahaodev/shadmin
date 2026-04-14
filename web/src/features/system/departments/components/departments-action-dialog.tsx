import type { Department } from '@/types/department'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
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
import { useDepartmentForm } from '../hooks/use-department-form'
import { useDepartmentTree } from '../hooks/use-departments'

interface DepartmentsActionDialogProps {
  currentRow?: Department | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function flattenForSelect(
  departments: Department[],
  excludeId?: string,
  level = 0
): { id: string; name: string; level: number }[] {
  const result: { id: string; name: string; level: number }[] = []
  for (const dept of departments) {
    if (dept.id === excludeId) continue
    result.push({ id: dept.id, name: dept.name, level })
    if (dept.children) {
      result.push(...flattenForSelect(dept.children, excludeId, level + 1))
    }
  }
  return result
}

export function DepartmentsActionDialog({
  currentRow,
  open,
  onOpenChange,
}: DepartmentsActionDialogProps) {
  const { form, onSubmit, isEdit, isSubmitting } = useDepartmentForm({
    currentRow,
    onSuccess: () => onOpenChange(false),
  })

  const { data: treeData } = useDepartmentTree()
  const parentOptions = flattenForSelect(
    treeData || [],
    isEdit ? currentRow?.id : undefined
  )

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        form.reset()
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-start'>
          <DialogTitle>{isEdit ? '编辑部门' : '添加部门'}</DialogTitle>
          <DialogDescription>
            {isEdit ? '修改部门信息。' : '创建新的部门。'}
            完成后点击保存。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            id='department-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='parent_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>上级部门</FormLabel>
                  <Select
                    onValueChange={(val) =>
                      field.onChange(val === '__none__' ? '' : val)
                    }
                    value={field.value || '__none__'}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder='无（顶级部门）' />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='__none__'>无（顶级部门）</SelectItem>
                      {parentOptions.map((opt) => (
                        <SelectItem key={opt.id} value={opt.id}>
                          {'　'.repeat(opt.level)}
                          {opt.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>部门名称</FormLabel>
                  <FormControl>
                    <Input placeholder='请输入部门名称' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='sequence'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>排序</FormLabel>
                    <FormControl>
                      <Input type='number' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>状态</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='active'>正常</SelectItem>
                        <SelectItem value='inactive'>停用</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='leader'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>负责人</FormLabel>
                  <FormControl>
                    <Input placeholder='请输入负责人' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='phone'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>联系电话</FormLabel>
                    <FormControl>
                      <Input placeholder='请输入联系电话' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='email'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>邮箱</FormLabel>
                    <FormControl>
                      <Input type='email' placeholder='请输入邮箱' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='department-form' disabled={isSubmitting}>
            {isSubmitting ? '保存中...' : '保存'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
