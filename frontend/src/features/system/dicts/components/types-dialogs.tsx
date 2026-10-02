import { useEffect } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createDictType,
  deleteDictType,
  updateDictType,
} from '@/services/dictApi'
import type { ApiError } from '@/types/api'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
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
import { Textarea } from '@/components/ui/textarea'
import { DICT_QUERY_KEYS } from '../hooks/dict-query-keys'
import { useDicts } from './dicts-provider'

type DictTypeStatus = 'active' | 'inactive'

const formSchema = z.object({
  code: z.string().min(1, '请输入类型编码'),
  name: z.string().min(1, '请输入类型名称'),
  status: z.enum(['active', 'inactive']),
  remark: z.string(),
})

type DictTypeForm = z.infer<typeof formSchema>

interface DictTypeFormFieldsProps {
  form: ReturnType<typeof useForm<DictTypeForm>>
}

function DictTypeFormFields({ form }: DictTypeFormFieldsProps) {
  return (
    <div className='space-y-4'>
      <FormField
        control={form.control}
        name='code'
        render={({ field }) => (
          <FormItem>
            <FormLabel>类型编码 *</FormLabel>
            <FormControl>
              <Input placeholder='请输入类型编码' {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name='name'
        render={({ field }) => (
          <FormItem>
            <FormLabel>类型名称 *</FormLabel>
            <FormControl>
              <Input placeholder='请输入类型名称' {...field} />
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
                  <SelectValue placeholder='选择状态' />
                </SelectTrigger>
              </FormControl>
              <SelectContent>
                <SelectItem value='active'>启用</SelectItem>
                <SelectItem value='inactive'>禁用</SelectItem>
              </SelectContent>
            </Select>
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={form.control}
        name='remark'
        render={({ field }) => (
          <FormItem>
            <FormLabel>备注</FormLabel>
            <FormControl>
              <Textarea placeholder='请输入备注' {...field} />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}

export function TypesDialogs() {
  const { typeOpen, setTypeOpen, currentTypeRow, setCurrentTypeRow } =
    useDicts()

  const queryClient = useQueryClient()

  const createForm = useForm<DictTypeForm>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      code: '',
      name: '',
      status: 'active',
      remark: '',
    },
  })

  const editForm = useForm<DictTypeForm>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      code: '',
      name: '',
      status: 'active',
      remark: '',
    },
  })

  const handleApiError = (error: ApiError, defaultMessage: string) => {
    toast.error(error?.response?.data?.msg || defaultMessage)
  }

  const refreshData = () => {
    queryClient.invalidateQueries({
      queryKey: [DICT_QUERY_KEYS.types],
    })
  }

  // 当 currentTypeRow 改变时，更新编辑表单的值
  useEffect(() => {
    if (currentTypeRow && typeOpen === 'edit') {
      editForm.reset({
        code: currentTypeRow.code,
        name: currentTypeRow.name,
        status: currentTypeRow.status as DictTypeStatus,
        remark: currentTypeRow.remark || '',
      })
    }
  }, [currentTypeRow, typeOpen, editForm])

  const createMutation = useMutation({
    mutationFn: createDictType,
    onSuccess: () => {
      setTypeOpen(null)
      createForm.reset()
      refreshData()
      toast.success('创建字典类型成功')
    },
    onError: (error: ApiError) => handleApiError(error, '创建失败'),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: DictTypeForm }) =>
      updateDictType(id, data),
    onSuccess: () => {
      setTypeOpen(null)
      setCurrentTypeRow(null)
      refreshData()
      toast.success('更新字典类型成功')
    },
    onError: (error: ApiError) => handleApiError(error, '更新失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDictType,
    onSuccess: () => {
      setTypeOpen(null)
      setCurrentTypeRow(null)
      refreshData()
      toast.success('删除字典类型成功')
    },
    onError: (error: ApiError) => handleApiError(error, '删除失败'),
  })

  const onCreateSubmit = (values: DictTypeForm) => {
    createMutation.mutate(values)
  }

  const onEditSubmit = (values: DictTypeForm) => {
    if (!currentTypeRow) return
    updateMutation.mutate({ id: currentTypeRow.id, data: values })
  }

  const handleDelete = () => {
    if (!currentTypeRow) return
    deleteMutation.mutate(currentTypeRow.id)
  }

  const handleEditDialogChange = (open: boolean) => {
    setTypeOpen(open ? 'edit' : null)
    if (!open) {
      setCurrentTypeRow(null)
    }
  }

  const handleCreateDialogChange = (open: boolean) => {
    setTypeOpen(open ? 'add' : null)
    if (!open) {
      createForm.reset()
    }
  }

  return (
    <>
      <Dialog open={typeOpen === 'add'} onOpenChange={handleCreateDialogChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>创建字典类型</DialogTitle>
            <DialogDescription>
              创建新的字典类型，用于分组管理字典项
            </DialogDescription>
          </DialogHeader>
          <Form {...createForm}>
            <form
              id='create-type-form'
              onSubmit={createForm.handleSubmit(onCreateSubmit)}
              className='space-y-4'
            >
              <DictTypeFormFields form={createForm} />
            </form>
          </Form>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant='outline' disabled={createMutation.isPending}>
                取消
              </Button>
            </DialogClose>
            <Button
              type='submit'
              form='create-type-form'
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? '创建中...' : '创建'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={typeOpen === 'edit'} onOpenChange={handleEditDialogChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>编辑字典类型</DialogTitle>
            <DialogDescription>修改字典类型信息</DialogDescription>
          </DialogHeader>
          <Form {...editForm}>
            <form
              id='edit-type-form'
              onSubmit={editForm.handleSubmit(onEditSubmit)}
              className='space-y-4'
            >
              <DictTypeFormFields form={editForm} />
            </form>
          </Form>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant='outline' disabled={updateMutation.isPending}>
                取消
              </Button>
            </DialogClose>
            <Button
              type='submit'
              form='edit-type-form'
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? '保存中...' : '保存'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 删除确认对话框 */}
      <AlertDialog
        open={typeOpen === 'delete'}
        onOpenChange={(open) => {
          if (!open) {
            setTypeOpen(null)
            setCurrentTypeRow(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除</AlertDialogTitle>
            <AlertDialogDescription>
              您确定要删除字典类型 "{currentTypeRow?.name}" 吗？
              <br />
              删除字典类型不会自动删除其下字典项；如存在字典项将无法删除该类型。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? '删除中...' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
