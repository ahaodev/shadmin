import { DepartmentsActionDialog } from './departments-action-dialog'
import { DepartmentsDeleteDialog } from './departments-delete-dialog'
import { useDepartments } from './departments-provider'

export function DepartmentsDialogs() {
  const {
    showCreateDialog,
    setShowCreateDialog,
    showEditDialog,
    setShowEditDialog,
    showDeleteDialog,
    setShowDeleteDialog,
    currentRow,
  } = useDepartments()

  return (
    <>
      <DepartmentsActionDialog
        open={showCreateDialog}
        onOpenChange={setShowCreateDialog}
      />

      <DepartmentsActionDialog
        open={showEditDialog}
        onOpenChange={setShowEditDialog}
        currentRow={currentRow}
      />

      <DepartmentsDeleteDialog
        open={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        department={currentRow}
      />
    </>
  )
}
