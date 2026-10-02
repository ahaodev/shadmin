import { apiClient } from '@/services/config'
import type {
  Department,
  CreateDepartmentRequest,
  UpdateDepartmentRequest,
} from '@/types/department'

type DepartmentResponse = Omit<
  Department,
  'created_at' | 'updated_at' | 'children'
> & {
  created_at: string
  updated_at: string
  children?: DepartmentResponse[]
}

function parseDepartment(department: DepartmentResponse): Department {
  return {
    ...department,
    created_at: new Date(department.created_at),
    updated_at: new Date(department.updated_at),
    children: department.children?.map(parseDepartment),
  }
}

// GET /system/department/tree - Get department tree
export async function getDepartmentTree(): Promise<Department[]> {
  const response = await apiClient.get('/api/v1/system/department/tree')
  return response.data.data.map(parseDepartment)
}

// POST /system/department - Create department
export async function createDepartment(
  data: CreateDepartmentRequest
): Promise<Department> {
  const response = await apiClient.post('/api/v1/system/department', data)
  return parseDepartment(response.data.data)
}

// PUT /system/department/:id - Update department
export async function updateDepartment(
  id: string,
  data: UpdateDepartmentRequest
): Promise<Department> {
  const response = await apiClient.put(`/api/v1/system/department/${id}`, data)
  return parseDepartment(response.data.data)
}

// DELETE /system/department/:id - Delete department
export async function deleteDepartment(id: string): Promise<void> {
  await apiClient.delete(`/api/v1/system/department/${id}`)
}
