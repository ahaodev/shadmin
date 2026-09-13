import {
  createContext,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  useContext,
  useState,
} from 'react'
import type { Department } from '@/types/department'
import useDialogState from '@/hooks/use-dialog-state'

type DepartmentsDialogType = 'add' | 'edit' | 'delete'

interface DepartmentsContext {
  open: DepartmentsDialogType | null
  setOpen: (str: DepartmentsDialogType | null) => void
  currentRow: Department | null
  setCurrentRow: Dispatch<SetStateAction<Department | null>>
}

const DepartmentsContext = createContext<DepartmentsContext | null>(null)

interface DepartmentsProviderProps {
  children: ReactNode
}

export function DepartmentsProvider({ children }: DepartmentsProviderProps) {
  const [open, setOpen] = useDialogState<DepartmentsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Department | null>(null)

  return (
    <DepartmentsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </DepartmentsContext>
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
