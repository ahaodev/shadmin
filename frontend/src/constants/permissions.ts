export const PERMISSIONS = {
  SYSTEM: {
    MENU: {
      ADD: 'system:menu:add',
      EDIT: 'system:menu:edit',
      DELETE: 'system:menu:delete',
    },
    ROLE: {
      ADD: 'system:role:add',
      EDIT: 'system:role:edit',
      DELETE: 'system:role:delete',
    },
    USER: {
      ADD: 'system:user:add',
      EDIT: 'system:user:edit',
      DELETE: 'system:user:delete',
      INVITE: 'system:user:invite',
    },
    LOGIN_LOG: {
      CLEAN: 'system:login_log:clean',
    },
    DEPARTMENT: {
      ADD: 'system:department:add',
      EDIT: 'system:department:edit',
      DELETE: 'system:department:delete',
    },
    DICT: {
      ADD_TYPE: 'system:dict:add_type',
      EDIT_TYPE: 'system:dict:edit_type',
      DELETE_TYPE: 'system:dict:delete_type',
      ADD_ITEM: 'system:dict:add_item',
      EDIT_ITEM: 'system:dict:edit_item',
      DELETE_ITEM: 'system:dict:delete_item',
    },
  },
} as const
