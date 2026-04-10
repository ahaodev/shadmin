import { type ColumnDef } from '@tanstack/react-table'
import { ChevronDown, ChevronRight, Edit, Settings, Trash2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { PERMISSIONS } from '@/constants/permissions'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export interface FlatDepartment {
  id: string
  parent_id: string | null
  name: string
  sequence: number
  leader: string
  phone: string
  email: string
  status: string
  level: number
  hasChildren: boolean
  hierarchyIndex: string
}

interface DepartmentColumnsProps {
  expanded: Record<string, boolean>
  onExpandToggle: (hierarchyIndex: string) => void
  onEditClick: (dept: FlatDepartment) => void
  onDeleteClick: (dept: FlatDepartment) => void
  hasPermission: (permission: string) => boolean
}

export function createDepartmentColumns({
  expanded,
  onExpandToggle,
  onEditClick,
  onDeleteClick,
  hasPermission,
}: DepartmentColumnsProps): ColumnDef<FlatDepartment, unknown>[] {
  return [
    {
      accessorKey: 'name',
      header: '部门名称',
      cell: ({ row }) => {
        const dept = row.original

        return (
          <div
            className='flex items-center gap-2'
            style={{ paddingLeft: `${dept.level * 24}px` }}
          >
            {dept.hasChildren ? (
              <Button
                variant='ghost'
                size='sm'
                className='h-6 w-6 p-0'
                onClick={(e) => {
                  e.stopPropagation()
                  onExpandToggle(dept.hierarchyIndex)
                }}
              >
                {expanded[dept.hierarchyIndex] ? (
                  <ChevronDown className='h-4 w-4' />
                ) : (
                  <ChevronRight className='h-4 w-4' />
                )}
              </Button>
            ) : (
              <div className='w-6' />
            )}
            <span
              className={cn(
                'font-medium',
                dept.level === 0 && 'font-semibold',
                dept.status === 'inactive' && 'line-through opacity-50'
              )}
            >
              {dept.name}
            </span>
          </div>
        )
      },
    },
    {
      accessorKey: 'leader',
      header: '负责人',
      cell: ({ row }) => {
        const leader = row.original.leader
        return leader ? (
          <span className='text-sm'>{leader}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
    },
    {
      accessorKey: 'phone',
      header: '联系电话',
      cell: ({ row }) => {
        const phone = row.original.phone
        return phone ? (
          <span className='text-sm'>{phone}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
    },
    {
      accessorKey: 'email',
      header: '邮箱',
      cell: ({ row }) => {
        const email = row.original.email
        return email ? (
          <span className='text-sm'>{email}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
    },
    {
      accessorKey: 'sequence',
      header: '排序',
      cell: ({ row }) => {
        return <span className='text-sm'>{row.original.sequence}</span>
      },
    },
    {
      accessorKey: 'status',
      header: '状态',
      cell: ({ row }) => {
        const status = row.original.status
        return (
          <Badge variant={status === 'active' ? 'default' : 'secondary'}>
            {status === 'active' ? '正常' : '停用'}
          </Badge>
        )
      },
    },
    {
      id: 'actions',
      header: '操作',
      cell: ({ row }) => {
        const dept = row.original
        const canEdit = hasPermission(PERMISSIONS.SYSTEM.DEPT.EDIT)
        const canDelete = hasPermission(PERMISSIONS.SYSTEM.DEPT.DELETE)

        if (!canEdit && !canDelete) return null

        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant='ghost' className='h-8 w-8 p-0'>
                <span className='sr-only'>打开菜单</span>
                <Settings className='h-4 w-4' />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end'>
              {canEdit && (
                <DropdownMenuItem onClick={() => onEditClick(dept)}>
                  <Edit className='mr-2 h-4 w-4' />
                  编辑
                </DropdownMenuItem>
              )}
              {canDelete && (
                <DropdownMenuItem
                  onClick={() => onDeleteClick(dept)}
                  className='text-destructive'
                >
                  <Trash2 className='mr-2 h-4 w-4' />
                  删除
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]
}
