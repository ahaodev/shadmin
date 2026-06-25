import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import type { Department } from '@/types/department'
import { type DepartmentFormData, departmentFormSchema } from '../data/schema'
import { useCreateDepartment, useUpdateDepartment } from './use-departments'

interface UseDepartmentFormProps {
  currentRow?: Department | null
  onSuccess: () => void
}

export function useDepartmentForm({
  currentRow,
  onSuccess,
}: UseDepartmentFormProps) {
  const isEdit = !!currentRow
  const createDepartment = useCreateDepartment()
  const updateDepartment = useUpdateDepartment()

  const form = useForm<DepartmentFormData>({
    resolver: zodResolver(departmentFormSchema),
    defaultValues: {
      parent_id: '',
      name: '',
      sequence: 0,
      leader: '',
      phone: '',
      email: '',
      status: 'active',
    },
  })

  // Backfill form with existing data when editing
  useEffect(() => {
    if (currentRow) {
      form.reset({
        parent_id: currentRow.parent_id || '',
        name: currentRow.name,
        sequence: currentRow.sequence,
        leader: currentRow.leader || '',
        phone: currentRow.phone || '',
        email: currentRow.email || '',
        status: currentRow.status,
      })
    } else {
      form.reset({
        parent_id: '',
        name: '',
        sequence: 0,
        leader: '',
        phone: '',
        email: '',
        status: 'active',
      })
    }
  }, [currentRow, form])

  const onSubmit = async (values: DepartmentFormData) => {
    try {
      if (isEdit) {
        await updateDepartment.mutateAsync({
          id: currentRow!.id,
          data: {
            parent_id: values.parent_id || null,
            name: values.name,
            sequence: values.sequence,
            leader: values.leader || undefined,
            phone: values.phone || undefined,
            email: values.email || undefined,
            status: values.status,
          },
        })
      } else {
        await createDepartment.mutateAsync({
          parent_id: values.parent_id || null,
          name: values.name,
          sequence: values.sequence,
          leader: values.leader || undefined,
          phone: values.phone || undefined,
          email: values.email || undefined,
          status: values.status,
        })
      }

      form.reset()
      onSuccess()
    } catch (error) {
      console.error('Error submitting department form:', error)
    }
  }

  const isSubmitting = createDepartment.isPending || updateDepartment.isPending

  return {
    form,
    onSubmit,
    isEdit,
    isSubmitting,
  }
}
