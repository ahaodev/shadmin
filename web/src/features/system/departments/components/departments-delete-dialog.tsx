import type { Department } from '@/types/department'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useDeleteDepartment } from '../hooks/use-departments'

interface DepartmentsDeleteDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  department: Department | null
}

export function DepartmentsDeleteDialog({
  open,
  onOpenChange,
  department,
}: DepartmentsDeleteDialogProps) {
  const deleteDepartment = useDeleteDepartment()

  const handleDelete = () => {
    if (department) {
      deleteDepartment.mutate(department.id, {
        onSuccess: () => onOpenChange(false),
      })
    }
  }

  if (!department) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>删除部门</DialogTitle>
          <DialogDescription>
            确定要删除部门 <strong>"{department.name}"</strong> 吗？
            此操作不可撤销。如果该部门下有子部门或用户，将无法删除。
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={deleteDepartment.isPending}
          >
            取消
          </Button>
          <Button
            variant='destructive'
            onClick={handleDelete}
            disabled={deleteDepartment.isPending}
          >
            {deleteDepartment.isPending ? '删除中...' : '删除部门'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
