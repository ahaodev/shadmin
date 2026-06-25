import { useMemo, useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
  type SortingState,
} from '@tanstack/react-table'
import type { Department } from '@/types/department'
import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { usePermission } from '@/hooks/usePermission'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useDepartmentTree } from '../hooks/use-departments'
import {
  createDepartmentColumns,
  type FlatDepartment,
} from './departments-columns'
import { useDepartments } from './departments-provider'

function flattenDepartments(
  departments: Department[],
  expanded: Record<string, boolean>,
  level = 0,
  parentIndex = ''
): FlatDepartment[] {
  const result: FlatDepartment[] = []

  departments.forEach((dept, index) => {
    const hierarchyIndex = parentIndex ? `${parentIndex}-${index}` : `${index}`
    const hasChildren = !!(dept.children && dept.children.length > 0)

    result.push({
      id: dept.id,
      parent_id: dept.parent_id,
      name: dept.name,
      sequence: dept.sequence,
      leader: dept.leader,
      phone: dept.phone,
      email: dept.email,
      status: dept.status,
      level,
      hasChildren,
      hierarchyIndex,
    })

    if (hasChildren && expanded[hierarchyIndex]) {
      result.push(
        ...flattenDepartments(
          dept.children!,
          expanded,
          level + 1,
          hierarchyIndex
        )
      )
    }
  })

  return result
}

function findDepartmentById(
  departments: Department[],
  id: string
): Department | null {
  for (const dept of departments) {
    if (dept.id === id) return dept
    if (dept.children) {
      const found = findDepartmentById(dept.children, id)
      if (found) return found
    }
  }
  return null
}

export function DepartmentsTable() {
  const { setCurrentRow, setShowEditDialog, setShowDeleteDialog } =
    useDepartments()
  const { hasPermission } = usePermission()
  const { data: treeData, isLoading, error } = useDepartmentTree()

  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [searchTerm, setSearchTerm] = useState('')

  const handleExpandToggle = (hierarchyIndex: string) => {
    setExpanded((prev) => ({
      ...prev,
      [hierarchyIndex]: !prev[hierarchyIndex],
    }))
  }

  const tableData = useMemo(() => {
    if (!treeData || treeData.length === 0) return []
    return flattenDepartments(treeData, expanded)
  }, [treeData, expanded])

  const filteredData = useMemo(() => {
    if (!searchTerm) return tableData
    return tableData.filter(
      (item) =>
        item.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        (item.leader || '').toLowerCase().includes(searchTerm.toLowerCase())
    )
  }, [tableData, searchTerm])

  const handleEditClick = (dept: FlatDepartment) => {
    if (!treeData) return
    const original = findDepartmentById(treeData, dept.id)
    if (original) {
      setCurrentRow(original)
      setShowEditDialog(true)
    }
  }

  const handleDeleteClick = (dept: FlatDepartment) => {
    if (!treeData) return
    const original = findDepartmentById(treeData, dept.id)
    if (original) {
      setCurrentRow(original)
      setShowDeleteDialog(true)
    }
  }

  const columns = createDepartmentColumns({
    expanded,
    onExpandToggle: handleExpandToggle,
    onEditClick: handleEditClick,
    onDeleteClick: handleDeleteClick,
    hasPermission,
  })

  const table = useReactTable({
    data: filteredData,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    onSortingChange: setSorting,
    state: { sorting },
  })

  if (isLoading) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='ml-2'>加载部门数据...</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className='text-muted-foreground flex h-32 items-center justify-center'>
        <span>加载部门失败: {error.message}</span>
      </div>
    )
  }

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <Input
          placeholder='搜索部门名称或负责人...'
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className='h-8 w-[150px] lg:w-[250px]'
        />
      </div>

      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext()
                        )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  className={cn(
                    'hover:bg-muted/50',
                    row.original.level === 0 && 'bg-muted/20 font-medium',
                    row.original.level === 1 && 'bg-muted/10',
                    row.original.level >= 2 && 'bg-muted/5'
                  )}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className='h-24 text-center'
                >
                  没有找到部门数据
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
