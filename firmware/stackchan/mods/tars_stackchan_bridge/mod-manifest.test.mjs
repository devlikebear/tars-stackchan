import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const modDir = dirname(fileURLToPath(import.meta.url))

test('bridge mod reads runtime settings from the mod config module', async () => {
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')
  const manifest = JSON.parse(await readFile(join(modDir, 'manifest.json'), 'utf8'))

  assert.match(source, /from\s+['"]mod\/config['"]/)
  assert.doesNotMatch(source, /from\s+['"]mc\/config['"]/)
  assert.equal(manifest.config.tarsStackchan.ledName, 'head')
  assert.equal(manifest.config.tarsStackchan.speechPathPrefix, '/api/tts?text=')
  assert.match(source, /bridgeConfig\.ledName \?\? ['"]head['"]/)
  assert.match(source, /bridgeConfig\.speechPathPrefix \?\? ['"]['"]/)
})

test('bridge mod owns launch so hardware setup UI touch probing is bypassed', async () => {
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.match(source, /function\s+onLaunch\s*\(/)
  assert.match(source, /onLaunch,?/)
})

test('bridge mod ships its retained HTTP service modules', async () => {
  const manifest = JSON.parse(await readFile(join(modDir, 'manifest.json'), 'utf8'))
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.equal(manifest.modules['tars-http-server-service'], './http-server-service')
  assert.equal(manifest.modules['tars-listen'], './listen')
  assert.equal(manifest.modules['http-server-service'], undefined)
  assert.match(source, /from\s+['"]tars-http-server-service['"]/)
  assert.doesNotMatch(source, /from\s+['"]http-server-service['"]/)
})

test('HTTP response headers never pass undefined values to Moddable Headers', async () => {
  const source = await readFile(join(modDir, 'http-server-service.js'), 'utf8')

  assert.doesNotMatch(source, /import Headers from ['"]headers['"]/)
  assert.doesNotMatch(source, /new Headers/)
  assert.match(source, /new Map/)
  assert.match(source, /value !== undefined && value !== null/)
  assert.match(source, /bodyLength/)
  assert.match(source, /bodyLength\.toString\(\)/)
  assert.doesNotMatch(source, /for \(const \[key, value\] of Object\.entries\(options\.headers\)\) {\n\s+headers\.set\(key, value\)/)
})

test('bridge mod ships the perception capture endpoints and module', async () => {
  const manifest = JSON.parse(await readFile(join(modDir, 'manifest.json'), 'utf8'))
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.ok(manifest.modules['*'].includes('./perception'))
  assert.match(source, /from\s+['"]\.\/perception['"]/)
  assert.match(source, /server\.get\('\/v1\/camera\/snapshot'/)
  assert.match(source, /server\.get\('\/v1\/audio\/clip'/)
  assert.match(source, /server\.get\('\/v1\/sensors'/)
  // Capture endpoints must be authenticated (only /v1/status is open).
  assert.match(source, /server\.get\('\/v1\/camera\/snapshot',\s*withAuth\(/)
  assert.match(source, /server\.get\('\/v1\/audio\/clip',\s*withAuth\(/)
  // Snapshot/clip return binary bodies, not JSON.
  assert.match(source, /c\.body\(jpeg,\s*'image\/jpeg'\)/)
  assert.match(source, /c\.body\(wav,\s*'audio\/wav'\)/)
})

test('perception capture is isolated from the pure request module', async () => {
  const core = await readFile(join(modDir, 'bridge-core.js'), 'utf8')
  // bridge-core stays unit-testable under plain Node: no native imports.
  assert.doesNotMatch(core, /embedded:io\//)
  const perception = await readFile(join(modDir, 'perception.js'), 'utf8')
  assert.match(perception, /from\s+['"]embedded:io\/image\/in\/camera['"]/)
  assert.match(perception, /from\s+['"]embedded:io\/audio\/in['"]/)
})

test('speech route responds without waiting for TTS playback to finish', async () => {
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.match(source, /server\.post\('\/v1\/speech'/)
  assert.match(source, /startSpeech\(robot, speechUtterance\(request\.text\), request\.volume\)/)
  assert.match(source, /encodeURIComponent\(text\)/)
  assert.doesNotMatch(source, /await\s+robot\.say/)
})
