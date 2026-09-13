import { MenusCreateDialog } from './menus-create-dialog'
import { MenusDeleteDialog } from './menus-delete-dialog'
import { useMenus } from './menus-provider'

export function MenusDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useMenus()

  return (
    <>
      {/* create 模式下 currentRow 作为父级菜单，因此不放入 currentRow 守卫内 */}
      <MenusCreateDialog
        key='menu-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
        menu={currentRow}
        mode='create'
      />

      {currentRow && (
        <>
          <MenusCreateDialog
            key={`menu-edit-${currentRow.id}`}
            open={open === 'edit'}
            onOpenChange={() => {
              setOpen('edit')
              setTimeout(() => setCurrentRow(null), 500)
            }}
            menu={currentRow}
            mode='edit'
          />

          <MenusDeleteDialog
            key={`menu-delete-${currentRow.id}`}
            open={open === 'delete'}
            onOpenChange={() => {
              setOpen('delete')
              setTimeout(() => setCurrentRow(null), 500)
            }}
            menu={currentRow}
          />
        </>
      )}
    </>
  )
}
