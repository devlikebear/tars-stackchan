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

test('speech route responds without waiting for TTS playback to finish', async () => {
  const source = await readFile(join(modDir, 'mod.js'), 'utf8')

  assert.match(source, /server\.post\('\/v1\/speech'/)
  assert.match(source, /startSpeech\(robot, speechUtterance\(request\.text\), request\.volume\)/)
  assert.match(source, /encodeURIComponent\(text\)/)
  assert.doesNotMatch(source, /await\s+robot\.say/)
})
