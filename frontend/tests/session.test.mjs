import assert from 'node:assert/strict'
import { after, beforeEach, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'
import axios, { AxiosError } from 'axios'
import { QueryClient } from '@tanstack/react-query'

const cookies = new Map()
globalThis.document = {
  get cookie() { return [...cookies].map(([key, value]) => `${key}=${value}`).join('; ') },
  set cookie(value) {
    const [pair] = value.split(';')
    const [key, token] = pair.split('=')
    if (value.includes('max-age=0')) cookies.delete(key)
    else cookies.set(key, token)
  },
}
globalThis.localStorage = {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {},
}
globalThis.window = { location: { protocol: 'http:', host: 'localhost', pathname: '/sign-in', search: '', hash: '', replace: () => {} } }
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
