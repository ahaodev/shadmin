import { DepartmentsActionDialog } from './departments-action-dialog'
import { DepartmentsDeleteDialog } from './departments-delete-dialog'
import { useDepartments } from './departments-provider'

export function DepartmentsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useDepartments()

  return (
    <>
      <DepartmentsActionDialog
        key='department-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
      />

      {currentRow && (
        <>
          <DepartmentsActionDialog
            key={`department-edit-${currentRow.id}`}
            open={open === 'edit'}
            onOpenChange={() => {
              setOpen('edit')
              setTimeout(() => setCurrentRow(null), 500)
            }}
            currentRow={currentRow}
          />

          <DepartmentsDeleteDialog
            key={`department-delete-${currentRow.id}`}
            open={open === 'delete'}
            onOpenChange={() => {
              setOpen('delete')
              setTimeout(() => setCurrentRow(null), 500)
            }}
            department={currentRow}
          />
        </>
      )}
    </>
  )
}
