import assert from 'node:assert/strict'
import { beforeEach, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

const storage = new Map()
const cookies = new Map()
globalThis.localStorage = {
  getItem: (key) => storage.get(key) ?? null,
  setItem: (key, value) => storage.set(key, String(value)),
  removeItem: (key) => storage.delete(key),
}
globalThis.document = {
  set cookie(value) {
    const [pair] = value.split(';')
    const [key, token] = pair.split('=')
    if (value.includes('max-age=0')) cookies.delete(key)
    else cookies.set(key, token)
  },
}
globalThis.window = {
  addEventListener: () => {},
  location: { protocol: 'https:', host: 'localhost' },
}

beforeEach(() => {
  storage.clear()
  cookies.clear()
  storage.set('accessToken', 'legacy-access')
  storage.set('access_token', 'older-access')
  storage.set('refreshToken', 'legacy-refresh')
  storage.set('theme', 'dark')
  cookies.set('accessToken', 'legacy-access')
  cookies.set('refreshToken', 'legacy-refresh')
})

async function loadAuth() {
  const server = await createServer({
    configFile: false,
    root: fileURLToPath(new URL('..', import.meta.url)),
    resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
    server: { middlewareMode: true, hmr: false, ws: false, watch: null },
  })
  try {
    const { useAuthStore } = await server.ssrLoadModule('/src/stores/auth-store.ts')
    const tokens = await server.ssrLoadModule('/src/lib/token-storage.ts')
    return { auth: useAuthStore.getState().auth, tokens }
  } finally { await server.close() }
}

for (const identity of [null, 'legacy-session']) {
  test(`startup clears legacy credentials with session marker ${identity}`, async () => {
    if (identity) storage.set('shadmin-session', identity)
    const { auth } = await loadAuth()
    assert.equal(auth.user, null)
    assert.equal(auth.accessToken, '')
    assert.equal(auth.refreshToken, '')
    for (const key of ['accessToken', 'access_token', 'refreshToken']) {
      assert.equal(storage.has(key), false)
    }
    assert.equal(cookies.size, 0)
    assert.equal(storage.get('theme'), 'dark')
    assert.equal(storage.get('shadmin-session') ?? null, identity)
  })
}

test('startup preserves the scoped session when legacy credentials remain', async () => {
  const accessToken = `header.${Buffer.from(JSON.stringify({ name: 'bob', exp: 9999999999 })).toString('base64url')}.signature`
  storage.set('shadmin-session', 'bob-session')
  storage.set('accessToken:bob-session', accessToken)
  storage.set('refreshToken:bob-session', 'bob-refresh')
  const { auth } = await loadAuth()
  assert.equal(auth.user.accountNo, 'bob')
  assert.equal(auth.accessToken, accessToken)
  assert.equal(auth.refreshToken, 'bob-refresh')
  assert.equal(storage.get('accessToken:bob-session'), accessToken)
  assert.equal(storage.get('refreshToken:bob-session'), 'bob-refresh')
  assert.equal(storage.has('accessToken'), false)
  assert.equal(storage.has('refreshToken'), false)
  assert.equal(storage.get('theme'), 'dark')
})

test('late legacy writes never restore credentials without a matching scoped session', async () => {
  const { tokens } = await loadAuth()
  storage.set('accessToken', 'late-legacy-access')
  storage.set('refreshToken', 'late-legacy-refresh')
  for (const identity of [null, 'signed-out-session']) {
    if (identity) storage.set('shadmin-session', identity)
    assert.equal(tokens.getAccessToken(), '')
    assert.equal(tokens.getRefreshToken(), '')
  }
})
