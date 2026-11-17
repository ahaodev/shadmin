import type { Menu } from '@/types/menu'

export interface MenuWithChildren extends Menu {
  children?: MenuWithChildren[]
}

export function buildMenuHierarchy(menus: Menu[]): MenuWithChildren[] {
  const cloned = menus.map((m) => ({
    ...m,
    children: [] as MenuWithChildren[] | undefined,
  }))
  const menuMap = new Map<string, MenuWithChildren>()

  cloned.forEach((menu) => {
    menuMap.set(menu.id, menu)
  })

  const sortMenus = (arr: MenuWithChildren[]) =>
    arr.slice().sort((a, b) => a.sequence - b.sequence)

  const buildChildren = (parentId: string | undefined): MenuWithChildren[] => {
    const children = cloned.filter((m) => m.parent_id === parentId)
    const sortedChildren = sortMenus(children)

    sortedChildren.forEach((child) => {
      const childChildren = buildChildren(child.id)
      if (childChildren.length > 0) {
        child.children = childChildren
      }
    })

    return sortedChildren
  }

  return buildChildren(undefined)
}

export interface TableMenuItem extends Menu {
  level: number
  parentIndex?: string
  isExpanded?: boolean
  hierarchyIndex: string
  hidden?: boolean
}

export function flattenMenus(
  items: MenuWithChildren[],
  expanded: Record<string, boolean>,
  level = 0,
  parentIndex = ''
): TableMenuItem[] {
  const result: TableMenuItem[] = []

  items.forEach((item, index) => {
    const currentIndex = parentIndex ? `${parentIndex}.${index}` : `${index}`
    const tableItem: TableMenuItem = {
      ...item,
      level,
      parentIndex: parentIndex || undefined,
      isExpanded: expanded[currentIndex] || false,
      hierarchyIndex: currentIndex,
      hidden: item.visible === 'hide' || item.status === 'inactive',
    }

    result.push(tableItem)

    if (item.children && item.children.length > 0 && expanded[currentIndex]) {
      result.push(
        ...flattenMenus(item.children, expanded, level + 1, currentIndex)
      )
    }
  })

  return result
}

export interface RoleMenuOption {
  id: string
  label: string
  disabled: boolean
  children?: RoleMenuOption[]
}

export function transformMenusForRoleSelection(
  menus: MenuWithChildren[]
): RoleMenuOption[] {
  const transform = (menuList: MenuWithChildren[]): RoleMenuOption[] => {
    return menuList.map((menu) => ({
      id: menu.id,
      label: menu.name,
      disabled: menu.status === 'inactive',
      children: menu.children ? transform(menu.children) : undefined,
    }))
  }

  return transform(menus)
}
