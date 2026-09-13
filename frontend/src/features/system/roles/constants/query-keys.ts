/**
 * 角色模块的 TanStack Query 缓存键。
 *
 * `ROLES_QUERY_KEY` 亦被 users 模块的角色下拉查询使用，因此必须在角色模块内
 * 单点定义，避免跨模块字面量漂移。
 */

/** 角色列表。 */
export const ROLES_QUERY_KEY = 'roles'

/** 角色创建/编辑对话框中的菜单选择数据源。 */
export const MENUS_FOR_ROLE_QUERY_KEY = 'menus-for-role'

/** 指定角色已勾选的菜单 id 列表。 */
export const ROLE_MENUS_QUERY_KEY = 'roleMenus'
