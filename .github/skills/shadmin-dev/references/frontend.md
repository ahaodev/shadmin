# Frontend Development Patterns

React/TypeScript patterns for Shadmin frontend. Read this when implementing frontend layers.

## Tech Stack

- **React 19** + TypeScript strict mode
- **Vite** + SWC + TanStack Router plugin (file-based routing, auto-generates `routeTree.gen.ts`)
- **TanStack Query** — data fetching, caching, mutations
- **TanStack Table** — headless table with server-side pagination/filtering
- **Zustand** — auth and permission state (`auth-store`)
- **React Hook Form** + **Zod** — form state and validation
- **Shadcn UI** (Radix + Tailwind v4) — UI primitives in `components/ui/` (do not edit)
- **Axios** — HTTP via `apiClient` singleton in `services/config.ts`
- **Sonner** — toast notifications via `toast.success` / `toast.error`
- **Lucide React** — icons

## Feature Module Structure

```
frontend/src/features/system/<resource>/
├── components/
│   ├── <resource>-provider.tsx        # Context: dialog state + current row
│   ├── <resource>-columns.tsx         # TanStack Table column definitions
│   ├── <resource>-table.tsx           # Data table with server-side pagination
│   ├── <resource>-dialogs.tsx         # Dialog aggregation (renders all dialogs)
│   ├── <resource>-primary-buttons.tsx # Header action buttons (permission-gated)
│   └── <resource>-form-dialog.tsx     # Create/edit form (single component handles both)
├── data/
│   └── schema.ts                      # Zod: data schema (API) + form schema (input)
├── hooks/
│   └── use-<resource>.ts              # TanStack Query hooks
└── index.tsx                          # Page entry component
```

## Types (`frontend/src/types/<resource>.ts`)

- Field names use `snake_case` to match backend domain structs
- Date fields are `Date` objects (not strings) — parsed in the API service
- Go pointer fields (`*string`) become optional TypeScript fields (`name?: string`)
- Define `CreateRequest`, `UpdateRequest`, `QueryParams` interfaces alongside the entity

## API Service (`frontend/src/services/<resource>Api.ts`)

- Import `apiClient` from `@/services/config`
- Always read `response.data.data` — outer `.data` is Axios, inner `.data` is `domain.Response.Data`
- Build query strings with `URLSearchParams`, using `snake_case` param names
- Parse string dates to `Date` objects via a `parseResource(raw)` converter function
- Endpoint pattern: `/api/v1/{module}/{resource}`

### `apiClient` behavior (`services/config.ts`)
- Base URL derived from `window.location` (works with Nginx reverse proxy)
- Auto-injects `Authorization: Bearer <token>` via request interceptor
- On 401: attempts token refresh, retries original request; on retry failure: redirects to login
- 300s timeout

## Context Provider (`components/<resource>-provider.tsx`)

Manages dialog state and selected row for the entire feature page.

- Dialog type is a string union: `'add' | 'edit' | 'delete' | null`
- Use `useDialogState<DialogType>(null)` hook from `@/hooks/use-dialog-state`
- Export both `<ResourceProvider>` and `useResource()` context hook
- `useResource()` throws if used outside provider

## TanStack Query Hooks (`hooks/use-<resource>.ts`)

- Query key: string constant (e.g., `const RESOURCE_KEY = 'resources'`)
- List hook: `queryKey: [RESOURCE_KEY, params]`, `staleTime: 5 * 60 * 1000`
- Single item hook: `enabled: !!id` to skip when no ID
- Mutations: `invalidateQueries({ queryKey: [RESOURCE_KEY] })` on success
- Error messages from `error?.response?.data?.msg` (backend `domain.Response.Msg`)
- Batch delete: `Promise.all(ids.map(deleteFn))`
- Toast on success/error via `sonner`

## Table (`components/<resource>-table.tsx`)

- `manualPagination: true` + `manualFiltering: true` — server handles both
- `pageCount = Math.ceil(totalCount / pageSize)`
- `useTableUrlState({ search, navigate })` syncs pagination and filters with URL
- `getRouteApi('/_authenticated/path')` for type-safe URL state access
- Columns include: checkbox select, data columns, actions column (permission-gated edit/delete)
- `DataTableRowActions` for row-level edit/delete; checks `hasPermission()` before rendering

## Dialog Aggregation (`components/<resource>-dialogs.tsx`)

- Renders all dialogs in one place at the bottom of the page
- String-based `open` state from provider determines which dialog is active
- Use unique `key` prop on each dialog to force remount when switching rows
- Edit dialog wrapped in `{currentRow && (...)}` to avoid rendering with stale data
- On close: `setTimeout(() => setCurrentRow(null), 500)` to delay cleanup until animation ends

## Form Dialog (`components/<resource>-form-dialog.tsx`)

- Single component handles both create and edit via `currentRow?: Resource` prop
- `isEdit = !!currentRow` determines title, submit behavior, and disabled fields
- `useEffect` resets the form when `currentRow` changes
- Combine `useCreateResource()` + `useUpdateResource()` mutations; `isSubmitting` from either `.isPending`
- Form: React Hook Form + Zod resolver; submit calls appropriate mutation then resets and closes

## Zod Schemas (`data/schema.ts`)

- **Data schema**: validates API responses — use `z.coerce.date()` for date fields
- **Form schema**: validates user input — use `.min(1, 'message')` for required fields
- Export `z.infer<typeof schema>` types alongside schemas

## Page Entry (`index.tsx`)

Structure: `<ResourceProvider>` wraps `<Header>` + `<Main>` + `<ResourceDialogs>`.

- `getRouteApi()` for type-safe search params
- Map URL params → API query params at the top of the component
- States: loading skeleton → error state → data table
- `<ResourceProvider>` must wrap the entire page for dialog context to work

## Route File (`frontend/src/routes/_authenticated/<path>.tsx`)

```typescript
const searchSchema = z.object({
  page:      z.number().optional().catch(1),
  page_size: z.number().optional().catch(10),
  search:    z.string().optional().catch(''),
  // add resource-specific filters here
})

export const Route = createFileRoute('/_authenticated/system/<resource>')({
  validateSearch: searchSchema,
  component: ResourcePage,
})
```

- `.catch(defaultValue)` provides safe fallbacks for malformed URL params
- Route path must match backend API structure
- Restart `pnpm dev` after adding a route to regenerate `routeTree.gen.ts`

## Permission Constants (`frontend/src/constants/permissions.ts`)

Add new permissions following the hierarchical pattern:
```typescript
SYSTEM: {
  RESOURCE: {
    ADD:    'system:resource:add',
    EDIT:   'system:resource:edit',
    DELETE: 'system:resource:delete',
  }
}
```

Usage: `const { hasPermission } = usePermission()` then `hasPermission(PERMISSIONS.SYSTEM.RESOURCE.ADD)`.

Admin users (`is_admin`) bypass all checks. Non-admin users are checked against the permissions array fetched from `/api/v1/resources` after login, stored in Zustand `auth-store`.

For component-level gating: use `<PermissionGuard>` or `<PermissionButton>` from `@/components/auth/`.

## Naming Conventions

| Type | Naming | Example |
|------|--------|---------|
| Components | PascalCase `.tsx` | `ProjectsTable.tsx` |
| Hooks | `use-kebab-case.ts` | `use-projects.ts` |
| Services | `camelCaseApi.ts` | `projectApi.ts` |
| Types | `kebab-case.ts` | `project.ts` |
| Route files | match URL path | `system/projects.tsx` |

## Import Rules

Always use the `@/` alias. Never use relative paths beyond `./` within a feature module.

Import order (enforced by Prettier plugin): React → third-party → Radix/form libs → TanStack → project aliases (`@/stores` → `@/lib` → `@/components` → `@/features`) → relative (`./`).
