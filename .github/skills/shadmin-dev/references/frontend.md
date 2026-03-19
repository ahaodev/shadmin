# Frontend Development Patterns

Detailed React/TypeScript code templates for Shadmin frontend features. Read this when implementing frontend layers.

## Tech Stack

- **React 19** + TypeScript (strict mode)
- **Vite** with SWC compiler + TanStack Router plugin
- **TanStack Router** — file-based routing under `web/src/routes/`
- **TanStack Query** — data fetching, caching, mutations
- **TanStack Table** — headless table with server-side pagination
- **Zustand** — lightweight global state (auth, permissions)
- **React Hook Form** + **Zod** — form validation
- **Shadcn UI** (Radix primitives + Tailwind CSS v4) — component library
- **Axios** — HTTP client via `apiClient` singleton in `services/config.ts`
- **Sonner** — toast notifications
- **Lucide React** — icons

## Types (`web/src/types/<resource>.ts`)

Align field names with backend `domain/` structs. Use `snake_case` for JSON fields:

```typescript
export interface Project {
  id: string
  name: string
  code: string
  description: string
  status: string
  created_at: Date
  updated_at: Date
}

export interface CreateProjectRequest {
  name: string
  code: string
  description?: string
  status?: string
}

// Pointer fields in Go become optional fields in TypeScript
export interface UpdateProjectRequest {
  name?: string
  code?: string
  description?: string
  status?: string
}

export interface ProjectQueryParams {
  page?: number
  page_size?: number
  name?: string
  status?: string
  search?: string
}
```

Import the shared `PagedResult` type or define locally:

```typescript
export interface PagedResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}
```

## API Service (`web/src/services/<resource>Api.ts`)

```typescript
import { apiClient } from '@/services/config'
import type { Project, CreateProjectRequest, UpdateProjectRequest, ProjectQueryParams } from '@/types/project'
import type { PagedResult } from '@/types/api'

// Date parser — convert API string dates to Date objects
const parseProject = (p: any): Project => ({
  ...p,
  created_at: new Date(p.created_at),
  updated_at: new Date(p.updated_at),
})

export const getProjects = async (params?: ProjectQueryParams): Promise<PagedResult<Project>> => {
  const searchParams = new URLSearchParams()
  if (params?.page) searchParams.append('page', params.page.toString())
  if (params?.page_size) searchParams.append('page_size', params.page_size.toString())
  if (params?.search) searchParams.append('search', params.search)
  if (params?.status) searchParams.append('status', params.status)
  if (params?.name) searchParams.append('name', params.name)

  const response = await apiClient.get(`/api/v1/system/project?${searchParams}`)
  const data = response.data.data  // outer .data = Axios, inner .data = domain.Response.Data
  return { ...data, list: (data.list || []).map(parseProject) }
}

export const getProject = async (id: string): Promise<Project> => {
  const response = await apiClient.get(`/api/v1/system/project/${id}`)
  return parseProject(response.data.data)
}

export const createProject = async (data: CreateProjectRequest): Promise<Project> => {
  const response = await apiClient.post('/api/v1/system/project', data)
  return parseProject(response.data.data)
}

export const updateProject = async (id: string, data: UpdateProjectRequest): Promise<void> => {
  await apiClient.put(`/api/v1/system/project/${id}`, data)
}

export const deleteProject = async (id: string): Promise<void> => {
  await apiClient.delete(`/api/v1/system/project/${id}`)
}
```

### API client (`services/config.ts`):
- Dynamic base URL from `window.location` (works with Nginx reverse proxy)
- Automatic `Authorization: Bearer <token>` injection via request interceptor
- Token stored in `localStorage` under `ACCESS_TOKEN` key
- 300s timeout

### Key conventions:
- `response.data.data` — always double `.data` (Axios wrapper + domain Response)
- `URLSearchParams` for query string construction
- Date fields parsed via a converter function
- Endpoint pattern: `/api/v1/{module}/{resource}`

## Feature Module Structure

```
web/src/features/system/projects/
├── components/
│   ├── projects-provider.tsx        # Context Provider (dialog state)
│   ├── projects-columns.tsx         # Table column definitions
│   ├── projects-table.tsx           # Data table with pagination
│   ├── projects-dialogs.tsx         # Dialog aggregation
│   ├── projects-primary-buttons.tsx # Header action buttons
│   └── project-form-dialog.tsx      # Create/edit form dialog
├── data/
│   └── schema.ts                    # Zod schemas (data + form validation)
├── hooks/
│   └── use-projects.ts              # TanStack Query hooks
└── index.tsx                        # Page entry component
```

## Context Provider (`components/<resource>-provider.tsx`)

Manages dialog open/close state and current row selection:

```tsx
import React, { useState, createContext, useContext } from 'react'
import { useDialogState } from '@/hooks/use-dialog-state'
import type { Project } from '@/types/project'

type ProjectsDialogType = 'add' | 'edit' | 'delete'

type ProjectsContextType = {
  open: ProjectsDialogType | null
  setOpen: (str: ProjectsDialogType | null) => void
  currentRow: Project | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Project | null>>
}

const ProjectsContext = createContext<ProjectsContextType | undefined>(undefined)

export function ProjectsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useDialogState<ProjectsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Project | null>(null)

  return (
    <ProjectsContext.Provider value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </ProjectsContext.Provider>
  )
}

export const useProjects = () => {
  const context = useContext(ProjectsContext)
  if (!context) {
    throw new Error('useProjects must be used within <ProjectsProvider>')
  }
  return context
}
```

## TanStack Query Hooks (`hooks/use-<resource>.ts`)

```typescript
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { getProjects, getProject, createProject, updateProject, deleteProject } from '@/services/projectApi'
import type { ProjectQueryParams, UpdateProjectRequest } from '@/types/project'

const PROJECTS_KEY = 'projects'

export function useProjectList(params?: ProjectQueryParams) {
  return useQuery({
    queryKey: [PROJECTS_KEY, params],
    queryFn: () => getProjects(params),
    staleTime: 5 * 60 * 1000,
  })
}

export function useProject(id: string) {
  return useQuery({
    queryKey: [PROJECTS_KEY, id],
    queryFn: () => getProject(id),
    enabled: !!id,
  })
}

export function useCreateProject() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createProject,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [PROJECTS_KEY] })
      toast.success('项目创建成功')
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || '创建失败')
    },
  })
}

export function useUpdateProject() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateProjectRequest }) =>
      updateProject(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [PROJECTS_KEY] })
      toast.success('项目更新成功')
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || '更新失败')
    },
  })
}

export function useDeleteProjects() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (ids: string[]) => Promise.all(ids.map(deleteProject)),
    onSuccess: (_, ids) => {
      queryClient.invalidateQueries({ queryKey: [PROJECTS_KEY] })
      toast.success(`已删除 ${ids.length} 个项目`)
    },
    onError: (error: any) => {
      toast.error(error?.response?.data?.msg || '删除失败')
    },
  })
}
```

### Key conventions:
- String constants for query keys
- `staleTime: 5 * 60 * 1000` as default
- `invalidateQueries` on mutation success to refetch lists
- `toast.success`/`toast.error` via sonner
- Error message from `error.response.data.msg` (backend's `domain.Response.Msg`)
- Batch delete via `Promise.all`

## Table Columns (`components/<resource>-columns.tsx`)

```tsx
import { ColumnDef } from '@tanstack/react-table'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { DataTableRowActions } from '@/components/data-table/data-table-row-actions'
import { LongText } from '@/components/long-text'
import type { Project } from '@/types/project'
import { useProjects } from './projects-provider'
import { usePermission } from '@/hooks/use-permission'
import { PERMISSIONS } from '@/constants/permissions'

export function useProjectColumns(): ColumnDef<Project>[] {
  const { setOpen, setCurrentRow } = useProjects()
  const { hasPermission } = usePermission()

  return [
    // Checkbox selection column
    {
      id: 'select',
      header: ({ table }) => (
        <Checkbox
          checked={table.getIsAllPageRowsSelected()}
          onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
          aria-label='Select all'
        />
      ),
      cell: ({ row }) => (
        <Checkbox
          checked={row.getIsSelected()}
          onCheckedChange={(value) => row.toggleSelected(!!value)}
          aria-label='Select row'
        />
      ),
      enableSorting: false,
      enableHiding: false,
    },
    // Data columns
    {
      accessorKey: 'name',
      header: '项目名称',
      cell: ({ row }) => <LongText className='max-w-36'>{row.getValue('name')}</LongText>,
    },
    {
      accessorKey: 'code',
      header: '项目编码',
    },
    {
      accessorKey: 'status',
      header: '状态',
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return <Badge variant={status === 'active' ? 'default' : 'secondary'}>{status}</Badge>
      },
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
    },
    {
      accessorKey: 'created_at',
      header: '创建时间',
    },
    // Actions column
    {
      id: 'actions',
      cell: ({ row }) => {
        const canEdit = hasPermission(PERMISSIONS.SYSTEM.PROJECT.EDIT)
        const canDelete = hasPermission(PERMISSIONS.SYSTEM.PROJECT.DELETE)
        if (!canEdit && !canDelete) return null

        return (
          <DataTableRowActions
            row={row}
            onEdit={canEdit ? () => { setCurrentRow(row.original); setOpen('edit') } : undefined}
            onDelete={canDelete ? () => { setCurrentRow(row.original); setOpen('delete') } : undefined}
          />
        )
      },
    },
  ]
}
```

## Data Table (`components/<resource>-table.tsx`)

```tsx
import { getRouteApi } from '@tanstack/react-router'
import { useReactTable, getCoreRowModel, getPaginationRowModel } from '@tanstack/react-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTable } from '@/components/data-table/data-table'
import { DataTablePagination } from '@/components/data-table/data-table-pagination'
import type { Project } from '@/types/project'
import { useProjectColumns } from './projects-columns'

const route = getRouteApi('/_authenticated/system/projects')

interface ProjectsTableProps {
  data: Project[]
  search: Record<string, unknown>
  navigate: (opts: any) => void
  totalCount: number
}

export function ProjectsTable({ data, search, navigate, totalCount }: ProjectsTableProps) {
  const columns = useProjectColumns()

  const { pagination, onPaginationChange, columnFilters, onColumnFiltersChange } =
    useTableUrlState({ search, navigate })

  const table = useReactTable({
    data,
    columns,
    pageCount: Math.ceil(totalCount / (pagination.pageSize || 10)),
    state: { pagination, columnFilters },
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,    // Server-side pagination
    manualFiltering: true,     // Server-side filtering
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  return (
    <div className='space-y-4'>
      <DataTable table={table} />
      <DataTablePagination table={table} />
    </div>
  )
}
```

### Key table patterns:
- `manualPagination: true` + `manualFiltering: true` — server handles these
- `pageCount` calculated from total / pageSize
- `useTableUrlState` syncs pagination and filters with URL search params
- `getRouteApi` connects to TanStack Router for type-safe URL state

## Dialog Aggregation (`components/<resource>-dialogs.tsx`)

```tsx
import { useProjects } from './projects-provider'
import { ProjectFormDialog } from './project-form-dialog'

export function ProjectsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useProjects()

  return (
    <>
      {/* Create dialog */}
      <ProjectFormDialog
        key='project-add'
        open={open === 'add'}
        onOpenChange={() => setOpen('add')}
      />
      {/* Edit dialog — only renders when currentRow is set */}
      {currentRow && (
        <ProjectFormDialog
          key={`project-edit-${currentRow.id}`}
          open={open === 'edit'}
          onOpenChange={() => {
            setOpen('edit')
            setTimeout(() => setCurrentRow(null), 500) // Cleanup after animation
          }}
          currentRow={currentRow}
        />
      )}
    </>
  )
}
```

### Key dialog patterns:
- String-based dialog type from provider (`'add' | 'edit' | 'delete'`)
- `key` prop forces remount on different entities
- 500ms setTimeout for cleanup after dialog close animation
- `currentRow` data passed only to edit/delete dialogs

## Primary Buttons (`components/<resource>-primary-buttons.tsx`)

```tsx
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { usePermission } from '@/hooks/use-permission'
import { PERMISSIONS } from '@/constants/permissions'
import { useProjects } from './projects-provider'

export function ProjectsPrimaryButtons() {
  const { setOpen } = useProjects()
  const { hasPermission } = usePermission()

  return (
    <div className='flex gap-2'>
      {hasPermission(PERMISSIONS.SYSTEM.PROJECT.ADD) && (
        <Button className='space-x-1' onClick={() => setOpen('add')}>
          <span>添加项目</span> <Plus size={18} />
        </Button>
      )}
    </div>
  )
}
```

## Form Dialog (`components/<resource>-form-dialog.tsx`)

```tsx
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import {
  Dialog, DialogClose, DialogContent, DialogDescription,
  DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import {
  Form, FormControl, FormField, FormItem, FormLabel, FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { Project } from '@/types/project'
import { useCreateProject, useUpdateProject } from '../hooks/use-projects'

const formSchema = z.object({
  name: z.string().min(1, '项目名称为必填项'),
  code: z.string().min(1, '项目编码为必填项'),
  description: z.string().optional(),
  status: z.string().default('active'),
})

type FormData = z.infer<typeof formSchema>

interface ProjectFormDialogProps {
  open: boolean
  onOpenChange: () => void
  currentRow?: Project
}

export function ProjectFormDialog({ open, onOpenChange, currentRow }: ProjectFormDialogProps) {
  const isEdit = !!currentRow
  const createMutation = useCreateProject()
  const updateMutation = useUpdateProject()

  const form = useForm<FormData>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: currentRow?.name ?? '',
      code: currentRow?.code ?? '',
      description: currentRow?.description ?? '',
      status: currentRow?.status ?? 'active',
    },
  })

  // Reset form when currentRow changes
  useEffect(() => {
    if (currentRow) {
      form.reset({
        name: currentRow.name,
        code: currentRow.code,
        description: currentRow.description,
        status: currentRow.status,
      })
    }
  }, [currentRow, form])

  const onSubmit = async (values: FormData) => {
    if (isEdit) {
      await updateMutation.mutateAsync({ id: currentRow!.id, data: values })
    } else {
      await createMutation.mutateAsync(values)
    }
    form.reset()
    onOpenChange()
  }

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? '编辑项目' : '添加项目'}</DialogTitle>
          <DialogDescription>
            {isEdit ? '修改项目信息。' : '填写项目信息创建新项目。'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>项目名称</FormLabel>
                  <FormControl>
                    <Input placeholder='输入项目名称' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='code'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>项目编码</FormLabel>
                  <FormControl>
                    <Input placeholder='输入项目编码' {...field} disabled={isEdit} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>描述</FormLabel>
                  <FormControl>
                    <Textarea placeholder='输入项目描述（可选）' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <DialogClose asChild>
                <Button type='button' variant='outline'>取消</Button>
              </DialogClose>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting ? '保存中...' : '保存'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
```

## Zod Schemas (`data/schema.ts`)

```typescript
import { z } from 'zod'

// Data schema — validates API responses
export const projectSchema = z.object({
  id: z.string(),
  name: z.string(),
  code: z.string(),
  description: z.string(),
  status: z.string(),
  created_at: z.coerce.date(),
  updated_at: z.coerce.date(),
})

export type ProjectSchema = z.infer<typeof projectSchema>

// Form schema — validates user input
export const projectFormSchema = z.object({
  name: z.string().min(1, '项目名称为必填项'),
  code: z.string().min(1, '项目编码为必填项'),
  description: z.string().optional(),
  status: z.string().default('active'),
  isEdit: z.boolean(),
})

export type ProjectFormData = z.infer<typeof projectFormSchema>
```

### Schema conventions:
- `z.coerce.date()` for date fields from API
- `.min(1, 'message')` for required fields with Chinese error messages
- Separate data schemas (API validation) from form schemas (input validation)
- `z.infer<typeof schema>` for type export

## Page Entry (`index.tsx`)

```tsx
import { getRouteApi } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Skeleton } from '@/components/ui/skeleton'
import { ProjectsProvider } from './components/projects-provider'
import { ProjectsPrimaryButtons } from './components/projects-primary-buttons'
import { ProjectsTable } from './components/projects-table'
import { ProjectsDialogs } from './components/projects-dialogs'
import { useProjectList } from './hooks/use-projects'

const route = getRouteApi('/_authenticated/system/projects')

export function Projects() {
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const queryParams = {
    page: search.page || 1,
    page_size: search.page_size || 10,
    search: search.search || undefined,
    status: search.status || undefined,
  }

  const { data, isLoading, error } = useProjectList(queryParams)

  return (
    <ProjectsProvider>
      <Header fixed>
        <Search />
        <div className='ms-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 space-y-2'>
          <div className='flex items-center justify-between'>
            <h2 className='text-2xl font-bold tracking-tight'>项目管理</h2>
            <ProjectsPrimaryButtons />
          </div>
          <p className='text-muted-foreground'>管理项目信息。</p>
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isLoading ? (
            <div className='space-y-4'>
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className='h-16 w-full' />
              ))}
            </div>
          ) : error ? (
            <div className='flex h-32 items-center justify-center text-muted-foreground'>
              加载失败，请重试
            </div>
          ) : (
            <ProjectsTable
              data={data?.list || []}
              search={search}
              navigate={(opts) => navigate(opts)}
              totalCount={data?.total || 0}
            />
          )}
        </div>
      </Main>

      <ProjectsDialogs />
    </ProjectsProvider>
  )
}
```

### Page layout pattern:
- `getRouteApi()` for type-safe URL search params
- URL params → API query params mapping
- `<ProjectsProvider>` wraps page for dialog state
- Header → Main → Dialogs structure
- Loading skeleton, error state, data state

## Route File (`web/src/routes/_authenticated/<path>.tsx`)

```typescript
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { Projects } from '@/features/system/projects'

const searchSchema = z.object({
  page: z.number().optional().catch(1),
  page_size: z.number().optional().catch(10),
  search: z.string().optional().catch(''),
  status: z.string().optional().catch(''),
})

export const Route = createFileRoute('/_authenticated/system/projects')({
  validateSearch: searchSchema,
  component: Projects,
})
```

### Route conventions:
- Zod schema validates URL search params with `.catch()` for safe defaults
- Component import from feature module's `index.tsx`
- Route path matches backend API structure
- After creating, restart `pnpm dev` to regenerate `routeTree.gen.ts`

## Permission Constants (`web/src/constants/permissions.ts`)

Add new permissions following the existing hierarchical pattern:

```typescript
export const PERMISSIONS = {
  SYSTEM: {
    // ... existing entries ...
    PROJECT: {
      ADD: 'system:project:add',
      EDIT: 'system:project:edit',
      DELETE: 'system:project:delete',
    },
  },
} as const
```

### Permission usage:
```typescript
import { usePermission } from '@/hooks/use-permission'
import { PERMISSIONS } from '@/constants/permissions'

const { hasPermission } = usePermission()

// In JSX — conditional rendering
{hasPermission(PERMISSIONS.SYSTEM.PROJECT.ADD) && <Button>Add</Button>}
```

### How `hasPermission` works (auth-store):
- Admin users (`is_admin`) bypass all checks automatically
- Non-admin: checks if the permission string exists in the user's permissions array
- Permissions fetched from `/api/v1/resources` endpoint after login

## Naming Conventions

| Type | Naming | Example |
|------|--------|---------|
| Components | PascalCase `.tsx` | `ProjectsTable.tsx` |
| Hooks | `use-kebab-case.ts` | `use-projects.ts` |
| Services | `camelCaseApi.ts` | `projectApi.ts` |
| Types | `kebab-case.ts` | `project.ts` |
| Route files | match URL path | `system/projects.tsx` |

## Import Rules

Always use the `@/` alias. Never use relative paths beyond `./` within a feature:

```typescript
// ✓ Correct
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'

// ✗ Wrong
import { cn } from '../../../lib/utils'
```

Import order (enforced by Prettier plugin):
1. React / Node modules
2. Third-party (zod, axios, date-fns)
3. Radix UI + form libraries
4. TanStack packages
5. Project aliases (`@/stores` → `@/lib` → `@/components` → `@/features`)
6. Relative imports (`./`)
