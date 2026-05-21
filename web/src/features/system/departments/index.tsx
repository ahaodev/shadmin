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

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>部门管理</h2>
            <p className='text-muted-foreground'>管理组织的部门结构</p>
          </div>
          <DepartmentsPrimaryButtons />
        </div>
        <DepartmentsTable />
      </Main>

      <DepartmentsDialogs />
    </DepartmentsProvider>
  )
}
