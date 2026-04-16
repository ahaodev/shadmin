import type { MenuTreeNode } from '@/types/menu'
import { useAuthStore } from '@/stores/auth-store'
import { BackendMenuAdapter } from '@/lib/backend-menu-adapter'
import type { NavGroup } from '@/components/layout/types'
import { getResourcesWithPermissions } from './resourceApi'

/**
 * Recursively extract all route paths from a menu tree.
 * Only collects paths from visible, active, non-button menus.
 */
function collectMenuPaths(menus: MenuTreeNode[]): string[] {
  const paths: string[] = []
  for (const menu of menus) {
    if (menu.visible === 'hide' || menu.status === 'inactive') continue
    if (menu.type === 'button') continue
    if (menu.path) {
      paths.push(menu.path)
    }
    if (menu.children && menu.children.length > 0) {
      paths.push(...collectMenuPaths(menu.children))
    }
  }
  return paths
}

/**
 * Check if a pathname matches an allowed path using segment-boundary logic.
 * Exact match or the pathname starts with allowedPath followed by '/'.
 */
function isPathMatch(pathname: string, allowedPath: string): boolean {
  if (pathname === allowedPath) return true
  return pathname.startsWith(allowedPath + '/')
}

/**
 * Menu service for loading and caching menu data from backend API
 */
export class MenuService {
  private cachedMenuData: NavGroup[] | null = null
  private cachedRawMenus: MenuTreeNode[] | null = null
  private menuLoadFailed: boolean = false
  private backendAdapter = new BackendMenuAdapter()
  private isLoading: boolean = false
  private loadPromise: Promise<NavGroup[]> | null = null

  /**
   * Load menu data from backend API
   * Fetches menu resources from the backend
   */
  async loadMenuData(): Promise<NavGroup[]> {
    // Return cached data if available
    if (this.cachedMenuData) {
      return this.cachedMenuData
    }

    // If already loading, return the existing promise
    if (this.isLoading && this.loadPromise) {
      return this.loadPromise
    }

    // Start loading
    this.isLoading = true
    this.loadPromise = this._doLoadMenuData()

    try {
      const result = await this.loadPromise
      return result
    } finally {
      this.isLoading = false
      this.loadPromise = null
    }
  }

  /**
   * Internal method to actually load menu data
   */
  private async _doLoadMenuData(): Promise<NavGroup[]> {
    try {
      // Fetch complete resources data including permissions
      const resourcesData = await getResourcesWithPermissions()

      // Set permissions to auth store
      if (resourcesData.permissions) {
        const { setPermissions } = useAuthStore.getState().auth
        setPermissions({
          permissions: resourcesData.permissions.map((p) => [p]), // Convert string[] to string[][]
          roles: resourcesData.roles ?? [],
          menus: resourcesData.menus.map((m) => m.id), // Extract menu IDs
          is_admin: resourcesData.is_admin ?? false,
        })
      }

      // Cache raw menu tree for route authorization
      this.cachedRawMenus = resourcesData.menus
      this.menuLoadFailed = false

      // Transform the backend data using the backend adapter
      const navGroups = this.backendAdapter.transformToNavGroups(
        resourcesData.menus
      )

      // Cache the transformed data
      this.cachedMenuData = navGroups

      return navGroups
    } catch (error) {
      console.error('Failed to load menu data from backend:', error)
      this.menuLoadFailed = true
      // Return empty array if backend fails
      return []
    }
  }

  /**
   * Check if a route path is allowed for the current user.
   * Returns true if admin, path matches a menu, or menu data failed to load.
   * Must be called after loadMenuData().
   */
  isPathAllowed(pathname: string): boolean {
    // If menu load failed, allow navigation (don't block on transient errors)
    if (this.menuLoadFailed) return true

    // If no raw menus cached yet, allow (data not loaded)
    if (!this.cachedRawMenus) return true

    // Admin bypasses all checks
    const { permissions } = useAuthStore.getState().auth
    if (permissions?.is_admin) return true

    // Collect all allowed paths from raw menu tree
    const allowedPaths = collectMenuPaths(this.cachedRawMenus)

    // Check if pathname matches any allowed path
    return allowedPaths.some((p) => isPathMatch(pathname, p))
  }

  /**
   * Get cached menu data synchronously
   */
  getCachedMenuData(): NavGroup[] | null {
    return this.cachedMenuData
  }

  /**
   * Clear cache and reload menu data
   */
  async reloadMenuData(): Promise<NavGroup[]> {
    this.cachedMenuData = null
    return this.loadMenuData()
  }

  /**
   * Clear cached menu data
   */
  clearCache(): void {
    const hadCache = this.cachedMenuData !== null
    console.log(
      'Clearing menu cache - had cache:',
      hadCache,
      'cached groups:',
      this.cachedMenuData?.length || 0
    )
    this.cachedMenuData = null
    this.cachedRawMenus = null
    this.menuLoadFailed = false
    this.isLoading = false
    this.loadPromise = null
  }
}

// Export singleton instance
export const menuService = new MenuService()
