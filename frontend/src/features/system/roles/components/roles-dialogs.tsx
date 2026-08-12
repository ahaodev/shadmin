import { RolesCreateDialog } from './roles-create-dialog'
import { RolesDeleteDialog } from './roles-delete-dialog'
import { useRoles } from './roles-provider'

export function RolesDialogs() {
  const {
    showCreateDialog,
    setShowCreateDialog,
    showEditDialog,
    setShowEditDialog,
    showDeleteDialog,
    setShowDeleteDialog,
    currentRow,
  } = useRoles()

  return (
    <>
      <RolesCreateDialog
        open={showCreateDialog}
        onOpenChange={setShowCreateDialog}
      />

      <RolesCreateDialog
        open={showEditDialog}
        onOpenChange={setShowEditDialog}
        role={currentRow}
      />

      <RolesDeleteDialog
        open={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        roleAssignment={currentRow}
      />
    </>
  )
}
