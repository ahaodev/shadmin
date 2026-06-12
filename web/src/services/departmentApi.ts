import { apiClient } from '@/services/config'
import type {
  Department,
  CreateDepartmentRequest,
  UpdateDepartmentRequest,
} from '@/types/department'

// GET /system/department/tree - Get department tree
export async function getDepartmentTree(): Promise<Department[]> {
  const response = await apiClient.get('/api/v1/system/department/tree')
  const payload = response.data?.data
  return Array.isArray(payload) ? payload : []
}

// POST /system/department - Create department
export async function createDepartment(
  data: CreateDepartmentRequest
): Promise<Department> {
  const response = await apiClient.post('/api/v1/system/department', data)
  return response.data.data
}

// GET /system/department/:id - Get department by ID
export async function getDepartment(id: string): Promise<Department> {
  const response = await apiClient.get(`/api/v1/system/department/${id}`)
  return response.data.data
}

// PUT /system/department/:id - Update department
export async function updateDepartment(
  id: string,
  data: UpdateDepartmentRequest
): Promise<Department> {
  const response = await apiClient.put(`/api/v1/system/department/${id}`, data)
  return response.data.data
}

// DELETE /system/department/:id - Delete department
export async function deleteDepartment(id: string): Promise<string> {
  const response = await apiClient.delete(`/api/v1/system/department/${id}`)
  return response.data.data
}
