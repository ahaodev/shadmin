import {
  createContext,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  useContext,
  useState,
} from 'react'
import type { Role } from '@/types/role'
import useDialogState from '@/hooks/use-dialog-state'

type RolesDialogType = 'add' | 'edit' | 'delete'

interface RolesContext {
  open: RolesDialogType | null
  setOpen: (str: RolesDialogType | null) => void
  currentRow: Role | null
  setCurrentRow: Dispatch<SetStateAction<Role | null>>
}

const RolesContext = createContext<RolesContext | null>(null)

interface RolesProviderProps {
  children: ReactNode
}

export function RolesProvider({ children }: RolesProviderProps) {
  const [open, setOpen] = useDialogState<RolesDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Role | null>(null)

  return (
    <RolesContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </RolesContext>
  )
}

export const useRoles = () => {
  const rolesContext = useContext(RolesContext)

  if (!rolesContext) {
    throw new Error('useRoles has to be used within <RolesProvider>')
  }

  return rolesContext
}
