import {
  createContext,
  type Dispatch,
  type ReactNode,
  type SetStateAction,
  useContext,
  useState,
} from 'react'
import type { DictItem, DictType } from '@/types/dict'
import useDialogState from '@/hooks/use-dialog-state'

type DictDialogType = 'add' | 'edit' | 'delete'

interface DictsContext {
  selectedType: DictType | null
  setSelectedType: Dispatch<SetStateAction<DictType | null>>

  typeOpen: DictDialogType | null
  setTypeOpen: (open: DictDialogType | null) => void
  currentTypeRow: DictType | null
  setCurrentTypeRow: Dispatch<SetStateAction<DictType | null>>

  itemOpen: DictDialogType | null
  setItemOpen: (open: DictDialogType | null) => void
  currentItemRow: DictItem | null
  setCurrentItemRow: Dispatch<SetStateAction<DictItem | null>>

  itemsListOpen: boolean
  setItemsListOpen: Dispatch<SetStateAction<boolean>>
}

const DictsContext = createContext<DictsContext | null>(null)

interface DictsProviderProps {
  children: ReactNode
}

export function DictsProvider({ children }: DictsProviderProps) {
  const [selectedType, setSelectedType] = useState<DictType | null>(null)
  const [typeOpen, setTypeOpen] = useDialogState<DictDialogType>(null)
  const [currentTypeRow, setCurrentTypeRow] = useState<DictType | null>(null)
  const [itemOpen, setItemOpen] = useDialogState<DictDialogType>(null)
  const [currentItemRow, setCurrentItemRow] = useState<DictItem | null>(null)
  const [itemsListOpen, setItemsListOpen] = useState(false)

  return (
    <DictsContext.Provider
      value={{
        selectedType,
        setSelectedType,
        typeOpen,
        setTypeOpen,
        currentTypeRow,
        setCurrentTypeRow,
        itemOpen,
        setItemOpen,
        currentItemRow,
        setCurrentItemRow,
        itemsListOpen,
        setItemsListOpen,
      }}
    >
      {children}
    </DictsContext.Provider>
  )
}

export const useDicts = () => {
  const dictsContext = useContext(DictsContext)

  if (!dictsContext) {
    throw new Error('useDicts has to be used within <DictsProvider>')
  }

  return dictsContext
}
