export interface Department {
  id: string
  parent_id: string | null
  name: string
  sequence: number
  leader: string
  phone: string
  email: string
  status: string
  children?: Department[]
  created_at: string
  updated_at: string
}

export interface CreateDepartmentRequest {
  parent_id?: string | null
  name: string
  sequence: number
  leader?: string
  phone?: string
  email?: string
  status?: string
}

export interface UpdateDepartmentRequest {
  parent_id?: string | null
  name?: string
  sequence?: number
  leader?: string
  phone?: string
  email?: string
  status?: string
}
