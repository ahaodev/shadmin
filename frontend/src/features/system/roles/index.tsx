import { getRouteApi } from '@tanstack/react-router'
import { Main } from '@/components/layout/main'
import { PageHeader } from '@/components/layout/page-header'
import { RolesDialogs } from './components/roles-dialogs'
import { RolesPrimaryButtons } from './components/roles-primary-buttons'
import { RolesProvider } from './components/roles-provider'
import { RolesTable } from './components/roles-table'

const route = getRouteApi('/_authenticated')

export function Roles() {
  const search = route.useSearch()
  const navigate = route.useNavigate()

  return (
    <RolesProvider>
      <PageHeader fixed />

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>角色管理</h2>
            <p className='text-muted-foreground'>管理用户角色和跨租户权限。</p>
          </div>
          <RolesPrimaryButtons />
        </div>
        <RolesTable data={[]} search={search} navigate={navigate} />
      </Main>

      <RolesDialogs />
    </RolesProvider>
  )
}
