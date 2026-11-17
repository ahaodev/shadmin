export interface RoleInfo {
  id: string
  name: string
  type?: string
  status?: 'active' | 'inactive'
  sequence?: number
  created_at?: string | Date
  updated_at?: string | Date
  menus?: string[]
  is_system?: boolean
}

export interface Role {
  id: string
  name: string
  sequence: number
  status: 'active' | 'inactive'
  created_at: Date
  updated_at: Date
  menu_ids?: string[]
}

export interface CreateRoleRequest {
  name: string
  sequence: number
  status?: 'active' | 'inactive'
  menu_ids?: string[]
}

export interface UpdateRoleRequest {
  name?: string
  sequence?: number
  status?: 'active' | 'inactive'
  menu_ids?: string[]
}

export interface RolePagedResult {
  list: Role[]
  total: number
  page: number
  page_size: number
  total_pages: number
}
