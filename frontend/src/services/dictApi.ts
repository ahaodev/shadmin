import {
  type DictType,
  type DictItem,
  type CreateDictTypeRequest,
  type UpdateDictTypeRequest,
  type CreateDictItemRequest,
  type UpdateDictItemRequest,
  type DictTypeQueryParams,
  type DictItemQueryParams,
  type DictTypePagedResult,
  type DictItemPagedResult,
} from '@/types/dict'
import { buildSearchParams } from '@/lib/query-params'
import { apiClient } from './config'

// Dictionary Type Management API

// Helpers to normalize date fields from API
const parseDictType = (t: DictType): DictType => ({
  ...t,
  created_at: new Date(t.created_at as unknown as string),
  updated_at: new Date(t.updated_at as unknown as string),
})

const parseDictItem = (i: DictItem): DictItem => ({
  ...i,
  created_at: new Date(i.created_at as unknown as string),
  updated_at: new Date(i.updated_at as unknown as string),
})

// GET /system/dict/types - Get dictionary types with pagination
export const getDictTypes = async (
  params?: DictTypeQueryParams
): Promise<DictTypePagedResult> => {
  const searchParams = buildSearchParams(params)

  const response = await apiClient.get(
    `/api/v1/system/dict/types?${searchParams}`
  )
  const data = response.data.data as DictTypePagedResult
  return {
    ...data,
    list: (data.list || []).map(parseDictType),
  }
}

// POST /system/dict/types - Create a new dictionary type
export const createDictType = async (
  data: CreateDictTypeRequest
): Promise<DictType> => {
  const response = await apiClient.post('/api/v1/system/dict/types', data)
  return parseDictType(response.data.data)
}

// PUT /system/dict/types/{id} - Update dictionary type
export const updateDictType = async (
  id: string,
  data: UpdateDictTypeRequest
): Promise<void> => {
  await apiClient.put(`/api/v1/system/dict/types/${id}`, data)
}

// DELETE /system/dict/types/{id} - Delete dictionary type
export const deleteDictType = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/dict/types/${id}`)
}

// Dictionary Item Management API

// GET /system/dict/items - Get dictionary items with pagination
export const getDictItems = async (
  params?: DictItemQueryParams
): Promise<DictItemPagedResult> => {
  const searchParams = buildSearchParams(params)

  const response = await apiClient.get(
    `/api/v1/system/dict/items?${searchParams}`
  )
  const data = response.data.data as DictItemPagedResult
  return {
    ...data,
    list: (data.list || []).map(parseDictItem),
  }
}

// POST /system/dict/items - Create a new dictionary item
export const createDictItem = async (
  data: CreateDictItemRequest
): Promise<DictItem> => {
  const response = await apiClient.post('/api/v1/system/dict/items', data)
  return parseDictItem(response.data.data)
}

// PUT /system/dict/items/{id} - Update dictionary item
export const updateDictItem = async (
  id: string,
  data: UpdateDictItemRequest
): Promise<void> => {
  await apiClient.put(`/api/v1/system/dict/items/${id}`, data)
}

// DELETE /system/dict/items/{id} - Delete dictionary item
export const deleteDictItem = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/dict/items/${id}`)
}

// Utility functions for common operations

// Get dictionary items for a specific type (by type ID)
export const getDictItemsByTypeId = async (
  typeId: string,
  params?: Omit<DictItemQueryParams, 'type_id'>
): Promise<DictItemPagedResult> => {
  return getDictItems({ ...params, type_id: typeId })
}

// Toggle dictionary item status
export const toggleDictItemStatus = async (
  id: string,
  currentStatus: 'active' | 'inactive'
): Promise<void> => {
  const newStatus = currentStatus === 'active' ? 'inactive' : 'active'
  await updateDictItem(id, { status: newStatus })
}

// Set dictionary item as default
export const setDictItemAsDefault = async (id: string): Promise<void> => {
  await updateDictItem(id, { is_default: true })
}
