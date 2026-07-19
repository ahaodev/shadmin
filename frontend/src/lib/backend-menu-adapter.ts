import { type MenuTreeNode } from '@/types/menu'
import { type NavGroup, type NavItem } from '@/components/layout/types'

/**
 * Shared render guard for backend menus.
 */
function isRenderableMenu(menu: MenuTreeNode) {
  if (menu.visible === 'hide' || menu.status === 'inactive') {
    return false
  }
  return menu.type !== 'button'
}

function visibleNavItems(menu: MenuTreeNode) {
  return (menu.children ?? [])
    .map(convertMenuNodeToNavItem)
    .filter((item): item is NavItem => item !== null)
}

function createSingleNavItem(menu: MenuTreeNode): NavItem {
  return {
    title: menu.name,
    icon: menu.icon,
    url: menu.path,
    is_frame: menu.is_frame,
  }
}

/**
 * Convert a single MenuTreeNode to NavItem
 */
function convertMenuNodeToNavItem(menu: MenuTreeNode): NavItem | null {
  if (!isRenderableMenu(menu)) {
    return null
  }

  if (menu.children && menu.children.length > 0) {
    const visibleChildren = visibleNavItems(menu)

    if (visibleChildren.length === 0 && menu.path) {
      return createSingleNavItem(menu)
    }

    if (visibleChildren.length === 0) {
      return null
    }

    return {
      title: menu.name,
      items: visibleChildren,
      icon: menu.icon,
    }
  }

  return createSingleNavItem(menu)
}

/**
 * BackendMenuAdapter class for converting backend menu data to frontend navigation structure
 */
export class BackendMenuAdapter {
  /**
   * Transform backend MenuTreeNode array to frontend NavGroup array
   */
  transformToNavGroups(menuNodes: MenuTreeNode[]): NavGroup[] {
    if (!menuNodes || menuNodes.length === 0) {
      return []
    }
    return menuNodes
      .map(this.convertToNavGroup)
      .filter((group): group is NavGroup => group !== null)
  }

  /**
   * Convert a single MenuTreeNode to NavGroup
   */
  private convertToNavGroup(menu: MenuTreeNode): NavGroup | null {
    if (!isRenderableMenu(menu)) {
      return null
    }

    if (menu.children && menu.children.length > 0) {
      const visibleItems = visibleNavItems(menu)

      if (visibleItems.length === 0 && menu.path) {
        return createSingleNavItem(menu)
      }

      if (visibleItems.length === 0) {
        return null
      }

      return {
        title: menu.name,
        icon: menu.icon,
        items: visibleItems,
      }
    }

    return {
      ...createSingleNavItem(menu),
    }
  }
}
