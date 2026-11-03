import { CanceledError } from 'axios'
import { getResources } from '@/services/resourceApi'
import type { NavGroup } from '@/types/layout'
import type { MenuTreeNode, ResourcesResponse } from '@/types/menu'
import { BackendMenuAdapter } from '@/lib/backend-menu-adapter'

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
class MenuService {
  private cachedMenuData: NavGroup[] | null = null
  private resources: ResourcesResponse | null = null
  private backendAdapter = new BackendMenuAdapter()
  private version = 0
  private loadPromise: Promise<NavGroup[]> | null = null

  async loadMenuData(): Promise<NavGroup[]> {
    if (this.cachedMenuData) return this.cachedMenuData
    if (this.loadPromise) return this.loadPromise

    const version = this.version
    const pending = this.load(version)
    this.loadPromise = pending
    try {
      return await pending
    } finally {
      // An obsolete loader must not clear a newer request.
      if (this.loadPromise === pending) this.loadPromise = null
    }
  }

  private async load(version: number): Promise<NavGroup[]> {
    const resources = await getResources()
    if (version !== this.version)
      throw new CanceledError('Menu cache invalidated')
    const navGroups = this.backendAdapter.transformToNavGroups(resources.menus)
    this.resources = resources
    this.cachedMenuData = navGroups
    return navGroups
  }

  getPermissions() {
    if (!this.resources) return null
    return {
      permissions: (this.resources.permissions ?? []).map((p) => [p]),
      roles: this.resources.roles ?? [],
      menus: this.resources.menus.map((m) => m.id),
      is_admin: this.resources.is_admin ?? false,
    }
  }

  isPathAllowed(pathname: string): boolean {
    if (!this.resources) return false
    if (this.resources.is_admin) return true
    return collectMenuPaths(this.resources.menus).some((p) =>
      isPathMatch(pathname, p)
    )
  }

  getCachedMenuData(): NavGroup[] | null {
    return this.cachedMenuData
  }

  clearCache(): void {
    this.version++
    this.cachedMenuData = null
    this.resources = null
    this.loadPromise = null
  }
}

export const menuService = new MenuService()
