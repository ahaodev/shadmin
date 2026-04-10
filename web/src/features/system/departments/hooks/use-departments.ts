import { useQuery } from '@tanstack/react-query'
import {
  createDepartment,
  deleteDepartment,
  getDepartmentTree,
  updateDepartment,
} from '@/services/departmentApi'
import type {
  CreateDepartmentRequest,
  UpdateDepartmentRequest,
} from '@/types/department'
import { useCrudMutation } from '@/hooks/use-crud-mutation'

const DEPARTMENTS_QUERY_KEY = 'departments'

export function useDepartmentTree() {
  return useQuery({
    queryKey: [DEPARTMENTS_QUERY_KEY],
    queryFn: getDepartmentTree,
    staleTime: 5 * 60 * 1000,
  })
}

export function useCreateDepartment() {
  return useCrudMutation({
    mutationFn: (data: CreateDepartmentRequest) => createDepartment(data),
    queryKeys: [[DEPARTMENTS_QUERY_KEY]],
    successMessage: '部门创建成功',
    errorMessage: '创建部门失败',
  })
}

export function useUpdateDepartment() {
  return useCrudMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateDepartmentRequest }) =>
      updateDepartment(id, data),
    queryKeys: [[DEPARTMENTS_QUERY_KEY]],
    successMessage: '部门更新成功',
    errorMessage: '更新部门失败',
  })
}

export function useDeleteDepartment() {
  return useCrudMutation({
    mutationFn: deleteDepartment,
    queryKeys: [[DEPARTMENTS_QUERY_KEY]],
    successMessage: '部门删除成功',
    errorMessage: '删除部门失败',
  })
}
