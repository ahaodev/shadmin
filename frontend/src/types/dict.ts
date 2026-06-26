import { z } from 'zod'

export const dictTypeSchema = z.object({
  id: z.string(),
  code: z.string(),
  name: z.string(),
  status: z.enum(['active', 'inactive']),
  remark: z.string().optional(),
  created_at: z.coerce.date(),
  updated_at: z.coerce.date(),
})
export type DictType = z.infer<typeof dictTypeSchema>

export const dictItemSchema = z.object({
  id: z.string(),
  type_id: z.string(),
  label: z.string(),
  value: z.string(),
  sort: z.number(),
  is_default: z.boolean(),
  status: z.enum(['active', 'inactive']),
  color: z.string().optional(),
  remark: z.string().optional(),
  created_at: z.coerce.date(),
  updated_at: z.coerce.date(),
})
export type DictItem = z.infer<typeof dictItemSchema>

export type DictStatus = DictType['status']

export interface CreateDictTypeRequest {
  code: string
  name: string
  status?: DictStatus
  remark?: string
}

export interface UpdateDictTypeRequest {
  code?: string
  name?: string
  status?: DictStatus
  remark?: string
}

export interface DictTypeQueryParams {
  page?: number
  page_size?: number
  code?: string
  name?: string
  status?: DictStatus
  search?: string
  sort_by?: string
  order?: 'asc' | 'desc'
}

export interface CreateDictItemRequest {
  type_id: string
  label: string
  value: string
  sort?: number
  is_default?: boolean
  status?: DictStatus
  color?: string
  remark?: string
}

export interface UpdateDictItemRequest {
  label?: string
  value?: string
  sort?: number
  is_default?: boolean
  status?: DictStatus
  color?: string
  remark?: string
}

export interface DictItemQueryParams {
  page?: number
  page_size?: number
  type_id?: string
  type_code?: string
  label?: string
  value?: string
  status?: DictStatus
  search?: string
  sort_by?: string
  order?: 'asc' | 'desc'
}

export interface DictTypePagedResult {
  list: DictType[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface DictItemPagedResult {
  list: DictItem[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

// Form state
export interface DictTypeFormData {
  code: string
  name: string
  status: DictStatus
  remark: string
}

export interface DictItemFormData {
  type_id: string
  label: string
  value: string
  sort: number
  is_default: boolean
  status: DictStatus
  color: string
  remark: string
}

// Dialog state
export type DictAction = 'create' | 'edit' | 'view'
export interface DictDialogState {
  open: boolean
  action: DictAction
  data?: DictType | DictItem
}
export interface DictTableSelection {
  selectedTypeId?: string
  selectedType?: DictType
}
