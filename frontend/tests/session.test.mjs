import assert from 'node:assert/strict'
import { after, beforeEach, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'
import axios, { AxiosError } from 'axios'
import { QueryClient } from '@tanstack/react-query'

const cookies = new Map()
globalThis.document = {
  head: { appendChild: () => {} },
  createElement: () => ({ styleSheet: {} }),
  get cookie() { return [...cookies].map(([key, value]) => `${key}=${value}`).join('; ') },
  set cookie(value) {
    const [pair] = value.split(';')
    const [key, token] = pair.split('=')
    if (value.includes('max-age=0')) cookies.delete(key)
    else cookies.set(key, token)
  },
}
const storage = new Map()
globalThis.localStorage = {
  getItem: (key) => (storage.has(key) ? storage.get(key) : null),
  setItem: (key, value) => { storage.set(key, String(value)) },
  removeItem: (key) => { storage.delete(key) },
}
const storageListeners = []
globalThis.window = { addEventListener: (_, fn) => storageListeners.push(fn), location: { protocol: 'http:', host: 'localhost', pathname: '/sign-in', search: '', hash: '', replace: () => {}, reload: () => {} } }
Object.defineProperty(globalThis.navigator, 'locks', { configurable: true, value: { request: async (_name, _options, fn) => fn() } })
// The refresh client captures the default adapter when the module is loaded.
let refreshAdapter = async () => { throw new Error('Unexpected refresh') }
const originalAdapter = axios.defaults.adapter
axios.defaults.adapter = (config) => refreshAdapter(config)
after(() => { axios.defaults.adapter = originalAdapter })
const server = await createServer({
  configFile: false,
  root: fileURLToPath(new URL('..', import.meta.url)),
  resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
  server: { middlewareMode: true, hmr: false, ws: false, watch: null },
})
after(() => server.close())
const { useAuthStore } = await server.ssrLoadModule('/src/stores/auth-store.ts')
const { apiClient } = await server.ssrLoadModule('/src/services/config.ts')
const { menuService } = await server.ssrLoadModule('/src/lib/menu-service.ts')
const { session } = await server.ssrLoadModule('/src/lib/session.ts')
const auth = () => useAuthStore.getState().auth
const token = (name) => `header.${Buffer.from(JSON.stringify({ name, exp: 9999999999 })).toString('base64url')}.signature`
const response = (config, data) => ({ data: { code: 0, data }, status: 200, statusText: 'OK', headers: {}, config })
const resources = (path) => ({ menus: [{ id: path, name: path, path, type: 'menu', status: 'active', visible: 'show', children: [] }], permissions: [path], roles: [], is_admin: false })
function deferred() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  return { promise, resolve }
}
beforeEach(() => auth().startSession(token('alice'), 'refresh-alice'))

test('sign-out clears query data, cancels in-flight work and isolates the next account', async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const unsubscribe = session.onReset(() => { void client.cancelQueries(); client.clear() })
  try {
    client.setQueryData(['users'], ['alice-private-data'])
    const started = deferred()
    const pending = deferred()
    apiClient.defaults.adapter = async (config) => { started.resolve(config.signal); await pending.promise; return response(config, ['alice']) }
    const query = client.fetchQuery({ queryKey: ['private'], queryFn: () => apiClient.get('/private') }).catch(() => {})
    const signal = await started.promise
    auth().startSession(token('bob'), 'refresh-bob')
    assert.equal(signal.aborted, true)
    assert.equal(client.getQueryData(['users']), undefined)
    pending.resolve()
    await query
    assert.equal(client.getQueryData(['private']), undefined)
    assert.equal(auth().user.accountNo, 'bob')
  } finally { unsubscribe(); client.clear() }
})

test('late menu and profile responses cannot restore the previous session', async () => {
  const started = deferred()
  const pending = deferred()
  let count = 0
  apiClient.defaults.adapter = async (config) => {
    if (++count === 2) started.resolve()
    await pending.promise
    return response(config, config.url.includes('resources') ? resources('/alice') : { id: 'alice', username: 'alice' })
  }
  const menus = auth().loadMenus().catch((error) => error)
  const profile = auth().fetchProfile()
  await started.promise
  auth().reset()
  pending.resolve()
  assert.ok((await menus) instanceof Error)
  await profile
  assert.equal(auth().permissions, null)
  assert.equal(auth().profile, null)
  assert.equal(menuService.getCachedMenuData(), null)
  assert.equal(menuService.isPathAllowed('/alice'), false)
})

test('menu failures reject navigation data and permit a later retry', async () => {
  apiClient.defaults.adapter = async () => { throw new Error('offline') }
  await assert.rejects(auth().loadMenus(), /offline/)
  assert.equal(menuService.isPathAllowed('/system/user'), false)
  apiClient.defaults.adapter = async (config) => response(config, resources('/system/user'))
  await auth().loadMenus()
  assert.equal(menuService.isPathAllowed('/system/user'), true)
  assert.equal(menuService.isPathAllowed('/system/role'), false)
  assert.deepEqual(auth().permissions.permissions, [['/system/user']])
})

test('cache invalidation prevents an old menu request from replacing newer permissions', async () => {
  const started = deferred()
  const pending = deferred()
  apiClient.defaults.adapter = async (config) => { started.resolve(); await pending.promise; return response(config, resources('/old')) }
  const old = auth().loadMenus().catch((error) => error)
  await started.promise
  auth().clearSidebarCache()
  apiClient.defaults.adapter = async (config) => response(config, resources('/new'))
  await auth().loadMenus()
  pending.resolve()
  assert.ok((await old) instanceof Error)
  assert.equal(menuService.isPathAllowed('/old'), false)
  assert.equal(menuService.isPathAllowed('/new'), true)
  assert.deepEqual(auth().permissions.permissions, [['/new']])
})

function unauthorized(config) {
  return new AxiosError('Expired', 'ERR_BAD_REQUEST', config, undefined, {
    ...response(config, null), status: 401,
  })
}

test('concurrent 401s share one refresh and synchronize store and persisted tokens', async () => {
  const started = deferred()
  const pending = deferred()
  let refreshes = 0
  refreshAdapter = async (config) => {
    refreshes++
    started.resolve()
    await pending.promise
    return response(config, { accessToken: token('alice-refreshed'), refreshToken: 'rotated' })
  }
  apiClient.defaults.adapter = async (config) => {
    if (!config._retry) throw unauthorized(config)
    return response(config, config.headers.Authorization)
  }
  const first = apiClient.get('/first')
  const second = apiClient.get('/second')
  await started.promise
  pending.resolve()
  const responses = await Promise.all([first, second])
  assert.equal(refreshes, 1)
  assert.equal(auth().accessToken, token('alice-refreshed'))
  assert.equal(auth().refreshToken, 'rotated')
  // 令牌只落在 localStorage 一处：不再写同名 cookie。
  assert.equal(cookies.has('accessToken'), false)
  assert.equal(cookies.has('refreshToken'), false)
  for (const result of responses) assert.equal(result.data.data, `Bearer ${auth().accessToken}`)
})

test('a refresh from an old session cannot overwrite or sign out the new account', async () => {
  const started = deferred()
  const pending = deferred()
  refreshAdapter = async (config) => {
    started.resolve()
    await pending.promise
    return response(config, { accessToken: token('obsolete'), refreshToken: 'obsolete' })
  }
  apiClient.defaults.adapter = async (config) => { throw unauthorized(config) }
  const request = apiClient.get('/old-session').catch((error) => error)
  await started.promise
  auth().startSession(token('bob'), 'refresh-bob')
  pending.resolve()
  assert.ok(axios.isCancel(await request))
  assert.equal(auth().accessToken, token('bob'))
  assert.equal(auth().refreshToken, 'refresh-bob')
})


test('profile refresh replaces cached data and ignores older in-flight results', async () => {
  apiClient.defaults.adapter = async (config) => response(config, { id: 'alice', email: 'old@example.com' })
  await auth().fetchProfile()
  const started = deferred()
  const pending = deferred()
  apiClient.defaults.adapter = async (config) => {
    started.resolve()
    await pending.promise
    return response(config, { id: 'alice', email: 'obsolete@example.com' })
  }
  const old = auth().fetchProfile(true)
  await started.promise
  apiClient.defaults.adapter = async (config) => response(config, { id: 'alice', email: 'new@example.com' })
  await auth().fetchProfile(true)
  pending.resolve()
  await old
  assert.equal(auth().profile.email, 'new@example.com')
  assert.equal(auth().isLoadingProfile, false)
})


test('authorization writes refresh cached menus and current button permissions', async () => {
  const { refreshPermissions } = await server.ssrLoadModule('/src/lib/refresh-permissions.ts')
  apiClient.defaults.adapter = async (config) => response(config, resources('/old-role'))
  await auth().loadMenus()
  apiClient.defaults.adapter = async (config) => response(config, resources('/new-role'))
  await refreshPermissions()
  assert.equal(menuService.isPathAllowed('/old-role'), false)
  assert.equal(menuService.isPathAllowed('/new-role'), true)
  assert.deepEqual(auth().permissions.permissions, [['/new-role']])
})

test('a remote account change cancels late work and clears identity caches before sending', async () => {
  const oldIdentity = localStorage.getItem('shadmin-session')
  const tokens = await server.ssrLoadModule('/src/lib/token-storage.ts')
  const client = new QueryClient()
  const off = session.onReset(() => { void client.cancelQueries(); client.clear() })
  try {
    client.setQueryData(['private'], 'alice')
    auth().setProfile({ id: 'alice' })
    auth().setPermissions({ permissions: [['alice']], roles: [], menus: [], is_admin: false })
    const started = deferred(), pending = deferred()
    apiClient.defaults.adapter = async (config) => { started.resolve(); await pending.promise; return response(config, 'alice') }
    const old = apiClient.get('/private').catch((e) => e)
    await started.promise
    localStorage.setItem('accessToken:remote-bob', token('bob'))
    localStorage.setItem('refreshToken:remote-bob', 'bob-refresh')
    localStorage.setItem('shadmin-session', 'remote-bob')
    // A refresh writer in another process can finish after the identity check.
    tokens.setAccessToken(token('obsolete-alice'), oldIdentity)
    assert.equal(tokens.getAccessToken(), token('bob'))
    const newRequest = await apiClient.get('/private').catch((e) => e)
    assert.ok(axios.isCancel(newRequest))
    pending.resolve()
    assert.ok(axios.isCancel(await old))
    assert.equal(auth().user.accountNo, 'bob')
    assert.equal(auth().profile, null)
    assert.equal(auth().permissions, null)
    assert.equal(client.getQueryData(['private']), undefined)
  } finally { off(); client.clear() }
})

test('refresh completed by another tab is reused without rotating or clearing caches', async () => {
  const revision = session.version
  const started = deferred(), pending = deferred()
  let refreshes = 0
  refreshAdapter = async () => { refreshes++; throw new Error('must reuse remote tokens') }
  const originalLocks = navigator.locks
  Object.defineProperty(navigator, 'locks', { configurable: true, value: {
    request: async (_name, _options, fn) => { started.resolve(); await pending.promise; return fn() },
  } })
  try {
    apiClient.defaults.adapter = async (config) => {
      if (!config._retry) throw unauthorized(config)
      return response(config, config.headers.Authorization)
    }
    const result = apiClient.get('/private')
    await started.promise
    localStorage.setItem(`accessToken:${localStorage.getItem('shadmin-session')}`, token('alice-remote-refresh'))
    localStorage.setItem(`refreshToken:${localStorage.getItem('shadmin-session')}`, 'remote-rotated')
    pending.resolve()
    assert.equal((await result).data.data, `Bearer ${token('alice-remote-refresh')}`)
    assert.equal(refreshes, 0)
    assert.equal(session.version, revision)
  } finally { Object.defineProperty(navigator, 'locks', { configurable: true, value: originalLocks }) }
})

test('HTTP sign-in and reset work without crypto.randomUUID', () => {
  const descriptor = Object.getOwnPropertyDescriptor(crypto, 'randomUUID')
  Object.defineProperty(crypto, 'randomUUID', { configurable: true, value: undefined })
  try {
    auth().startSession(token('http-user'), 'http-refresh')
    const identity = localStorage.getItem('shadmin-session')
    assert.ok(identity)
    assert.equal(auth().user.accountNo, 'http-user')
    assert.equal(auth().accessToken, token('http-user'))
    assert.equal(auth().reset(), true)
    assert.notEqual(localStorage.getItem('shadmin-session'), identity)
    assert.equal(auth().accessToken, '')
    assert.equal(auth().refreshToken, '')
    assert.equal(auth().user, null)
  } finally {
    if (descriptor) Object.defineProperty(crypto, 'randomUUID', descriptor)
    else delete crypto.randomUUID
  }
})

test('late logout completion preserves a remote session with or without a storage event', async () => {
  const { logout } = await server.ssrLoadModule('/src/services/authApi.ts')
  for (const delivered of [false, true]) {
    auth().startSession(token('alice'), 'refresh-alice')
    const originalAuth = auth()
    const version = session.version
    const started = deferred(), pending = deferred()
    let navigated = false
    apiClient.defaults.adapter = async (config) => {
      started.resolve()
      await pending.promise
      return response(config, null)
    }
    const signingOut = (async () => {
      try { await logout() } catch {} finally {
        if (originalAuth.reset(version)) navigated = true
      }
    })()
    await started.promise
    const identity = `bob-during-logout-${delivered}`
    localStorage.setItem(`accessToken:${identity}`, token('bob'))
    localStorage.setItem(`refreshToken:${identity}`, 'bob-refresh')
    localStorage.setItem('shadmin-session', identity)
    if (delivered) storageListeners.forEach((fn) => fn({ key: 'shadmin-session' }))
    pending.resolve()
    await signingOut
    assert.equal(navigated, false)
    assert.equal(localStorage.getItem('shadmin-session'), identity)
    assert.equal(localStorage.getItem(`accessToken:${identity}`), token('bob'))
    assert.equal(localStorage.getItem(`refreshToken:${identity}`), 'bob-refresh')
    assert.equal(auth().user.accountNo, 'bob')
  }
})

test('scoped cleanup checks shared identity before a storage event arrives', () => {
  const version = session.version
  localStorage.setItem('accessToken:bob-before-event', token('bob'))
  localStorage.setItem('refreshToken:bob-before-event', 'bob-refresh')
  localStorage.setItem('shadmin-session', 'bob-before-event')
  assert.equal(auth().reset(version), false)
  assert.equal(localStorage.getItem('shadmin-session'), 'bob-before-event')
  assert.equal(auth().accessToken, token('bob'))
})

test('scoped cleanup clears its own session but ignores a newer local sign-in', () => {
  const version = session.version
  assert.equal(auth().reset(version), true)
  assert.equal(auth().accessToken, '')
  auth().startSession(token('bob'), 'bob-refresh')
  const identity = localStorage.getItem('shadmin-session')
  assert.equal(auth().reset(version), false)
  assert.equal(localStorage.getItem('shadmin-session'), identity)
  assert.equal(auth().accessToken, token('bob'))
})

test('refresh without Web Locks requires a new sign-in instead of consuming tokens', async () => {
  const originalLocks = navigator.locks
  let refreshes = 0
  Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined })
  refreshAdapter = async () => { refreshes++; throw new Error('must not refresh without a lock') }
  apiClient.defaults.adapter = async (config) => { throw unauthorized(config) }
  try {
    await assert.rejects(apiClient.get('/private'), /Token refresh requires Web Locks/)
    assert.equal(refreshes, 0)
    assert.equal(auth().user, null)
    assert.equal(auth().accessToken, '')
    assert.equal(auth().refreshToken, '')
  } finally { Object.defineProperty(navigator, 'locks', { configurable: true, value: originalLocks }) }
})
