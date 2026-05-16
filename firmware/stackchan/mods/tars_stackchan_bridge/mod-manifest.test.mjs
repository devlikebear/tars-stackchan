import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const modDir = dirname(fileURLToPath(import.meta.url))

test('bridge mod reads runtime settings from the mod config module', async () => {
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.match(source, /from\s+['"]mod\/config['"]/)
  assert.doesNotMatch(source, /from\s+['"]mc\/config['"]/)
})

test('bridge mod owns launch so hardware setup UI touch probing is bypassed', async () => {
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.match(source, /function\s+onLaunch\s*\(/)
  assert.match(source, /onLaunch,?/)
})

test('bridge mod ships its retained HTTP service modules', async () => {
  const manifest = JSON.parse(await readFile(join(modDir, 'manifest.json'), 'utf8'))

  assert.equal(manifest.modules['http-server-service'], './http-server-service')
  assert.equal(manifest.modules['tars-listen'], './listen')
})
