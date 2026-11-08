import assert from 'node:assert/strict'
import { after, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

const server = await createServer({
  configFile: false,
  root: fileURLToPath(new URL('..', import.meta.url)),
  resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
  server: { middlewareMode: true, hmr: false, ws: false, watch: null },
})
after(() => server.close())
const { isPasswordLengthValid } = await server.ssrLoadModule('/src/lib/password.ts')
const { userFormSchema } = await server.ssrLoadModule('/src/features/system/users/data/schema.ts')

test('new passwords enforce character minimum and UTF-8 byte maximum', () => {
  for (const [password, valid] of [
    ['', false], ['1234567', false], ['12345678', true],
    ['x'.repeat(72), true], ['x'.repeat(73), false],
    ['密码a1', false], ['密'.repeat(7), false], ['密'.repeat(8), true],
    ['密'.repeat(24), true], ['密'.repeat(25), false],
    ['🔑'.repeat(7), false], ['🔑'.repeat(8), true],
    ['🔑'.repeat(18), true], ['🔑'.repeat(19), false],
  ]) assert.equal(isPasswordLengthValid(password), valid)
})

test('user forms reject long passwords and preserve empty edit semantics', () => {
  const user = { username: 'alice', email: 'alice@example.com', status: 'active', roles: [], isEdit: false }
  const parse = (password, isEdit = false) => userFormSchema.safeParse({ ...user, password, confirmPassword: password, isEdit })
  assert.equal(parse('a1' + '密'.repeat(23)).success, true)
  assert.equal(parse('a1' + '密'.repeat(24)).success, false)
  assert.equal(parse('').success, false)
  assert.equal(parse('', true).success, true)
})
