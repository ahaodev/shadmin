/**
 * 菜单模块的 TanStack Query 缓存键。
 *
 * 键值在同一 feature 的多个文件间共享（列表查询、表单、拖拽排序、删除对话框），
 * 集中定义以避免字面量拼写漂移导致失效调用落空。
 */

/** 菜单列表。 */
export const MENUS_QUERY_KEY = 'menus'

/** 父级菜单选项（由菜单树扁平化而来）。 */
export const PARENT_MENUS_QUERY_KEY = 'parent-menus'
