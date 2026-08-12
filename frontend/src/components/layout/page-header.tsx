import { ConfigDrawer } from '@/components/config-drawer'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Header } from './header'

type PageHeaderProps = {
  fixed?: boolean
  /**
   * Optional content rendered on the left side of the header,
   * e.g. `<TopNav />` on the dashboard.
   */
  children?: React.ReactNode
}

/**
 * Shared header for authenticated pages: sidebar trigger + search +
 * theme/config/profile controls. Pages should not re-implement the
 * right-side control cluster.
 */
export function PageHeader({ fixed, children }: PageHeaderProps) {
  return (
    <Header fixed={fixed}>
      {children}
      <div className='ms-auto flex items-center space-x-4'>
        <Search />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </div>
    </Header>
  )
}
