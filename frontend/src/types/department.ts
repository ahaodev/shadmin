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
  created_at: Date
  updated_at: Date
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
