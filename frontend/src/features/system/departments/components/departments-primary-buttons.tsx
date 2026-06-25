import { Plus } from 'lucide-react'
import { PERMISSIONS } from '@/constants/permissions'
import { usePermission } from '@/hooks/usePermission'
import { Button } from '@/components/ui/button'
import { useDepartments } from './departments-provider'

export function DepartmentsPrimaryButtons() {
  const { setShowCreateDialog, setCurrentRow } = useDepartments()
  const { hasPermission } = usePermission()

  const handleCreateClick = () => {
    setCurrentRow(null)
    setShowCreateDialog(true)
  }

  return (
    <div className='flex space-x-2'>
      {hasPermission(PERMISSIONS.SYSTEM.DEPT.ADD) && (
        <Button onClick={handleCreateClick} className='space-x-1'>
          <span>添加部门</span>
          <Plus className='ml-1 h-4 w-4' />
        </Button>
      )}
    </div>
  )
}
