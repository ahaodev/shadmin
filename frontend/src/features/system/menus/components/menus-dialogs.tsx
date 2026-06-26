import { MenusCreateDialog } from './menus-create-dialog'
import { MenusDeleteDialog } from './menus-delete-dialog'
import { useMenus } from './menus-provider'

export function MenusDialogs() {
  const {
    showCreateDialog,
    setShowCreateDialog,
    showEditDialog,
    setShowEditDialog,
    showDeleteDialog,
    setShowDeleteDialog,
    currentRow,
  } = useMenus()
  return (
    <>
      <MenusCreateDialog
        open={showCreateDialog}
        onOpenChange={setShowCreateDialog}
        menu={currentRow}
        mode='create'
      />

      <MenusCreateDialog
        open={showEditDialog}
        onOpenChange={setShowEditDialog}
        menu={currentRow}
        mode='edit'
      />

      <MenusDeleteDialog
        open={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        menu={currentRow}
      />
    </>
  )
}
