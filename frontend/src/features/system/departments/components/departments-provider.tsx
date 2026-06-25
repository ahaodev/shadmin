import {
  createContext,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  useContext,
  useState,
} from 'react'
import type { Department } from '@/types/department'

interface DepartmentsContext {
  currentRow: Department | null
  setCurrentRow: Dispatch<SetStateAction<Department | null>>
  showCreateDialog: boolean
  setShowCreateDialog: Dispatch<SetStateAction<boolean>>
  showEditDialog: boolean
  setShowEditDialog: Dispatch<SetStateAction<boolean>>
  showDeleteDialog: boolean
  setShowDeleteDialog: Dispatch<SetStateAction<boolean>>
}

const DepartmentsContext = createContext<DepartmentsContext | null>(null)

interface DepartmentsProviderProps {
  children: ReactNode
}

export function DepartmentsProvider({ children }: DepartmentsProviderProps) {
  const [currentRow, setCurrentRow] = useState<Department | null>(null)
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [showEditDialog, setShowEditDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  return (
    <DepartmentsContext.Provider
      value={{
        currentRow,
        setCurrentRow,
        showCreateDialog,
        setShowCreateDialog,
        showEditDialog,
        setShowEditDialog,
        showDeleteDialog,
        setShowDeleteDialog,
      }}
    >
      {children}
    </DepartmentsContext.Provider>
  )
}

export const useDepartments = () => {
  const context = useContext(DepartmentsContext)
  if (!context) {
    throw new Error(
      'useDepartments has to be used within <DepartmentsProvider>'
    )
  }
  return context
}
