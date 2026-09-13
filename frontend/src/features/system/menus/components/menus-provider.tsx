import {
  createContext,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  useContext,
  useState,
} from 'react'
import type { Menu } from '@/types/menu'
import useDialogState from '@/hooks/use-dialog-state'

type MenusDialogType = 'add' | 'edit' | 'delete'

interface MenusContext {
  open: MenusDialogType | null
  setOpen: (str: MenusDialogType | null) => void
  currentRow: Menu | null
  setCurrentRow: Dispatch<SetStateAction<Menu | null>>
}

const MenusContext = createContext<MenusContext | null>(null)

interface MenusProviderProps {
  children: ReactNode
}

export function MenusProvider({ children }: MenusProviderProps) {
  const [open, setOpen] = useDialogState<MenusDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Menu | null>(null)

  return (
    <MenusContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </MenusContext>
  )
}

export const useMenus = () => {
  const menusContext = useContext(MenusContext)

  if (!menusContext) {
    throw new Error('useMenus has to be used within <MenusProvider>')
  }

  return menusContext
}
