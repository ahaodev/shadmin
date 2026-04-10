import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { DepartmentsDialogs } from './components/departments-dialogs'
import { DepartmentsPrimaryButtons } from './components/departments-primary-buttons'
import { DepartmentsProvider } from './components/departments-provider'
import { DepartmentsTable } from './components/departments-table'

export function Departments() {
  return (
    <DepartmentsProvider>
      <Header fixed>
        <Search />
        <div className='ms-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ConfigDrawer />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>部门管理</h2>
            <p className='text-muted-foreground'>管理组织的部门结构</p>
          </div>
          <DepartmentsPrimaryButtons />
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          <DepartmentsTable />
        </div>
      </Main>

      <DepartmentsDialogs />
    </DepartmentsProvider>
  )
}
