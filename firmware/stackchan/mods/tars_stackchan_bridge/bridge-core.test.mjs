import assert from 'node:assert/strict'
import test from 'node:test'

import {
  actionResponse,
  clampTiltDeg,
  isAuthorized,
  normalizeExpressionRequest,
  normalizeHeadRequest,
  normalizeLEDRequest,
  normalizeMotionRequest,
  normalizeSpeechRequest,
} from './bridge-core.js'

test('bearer token auth requires an exact configured token match', () => {
  assert.equal(isAuthorized('Bearer secret', 'secret'), true)
  assert.equal(isAuthorized('bearer secret', 'secret'), false)
  assert.equal(isAuthorized('Bearer wrong', 'secret'), false)
  assert.equal(isAuthorized(undefined, 'secret'), false)
  assert.throws(() => isAuthorized('Bearer anything', ''), /token is not configured/)
})

test('head request clamps tilt and converts to robot rotation', () => {
  const high = normalizeHeadRequest({ pan_deg: 120, tilt_deg: 120, speed: 0.6 })

  assert.equal(high.panDeg, 90)
  assert.equal(high.tiltDeg, 85)
  assert.equal(high.speed, 0.6)
  assert.equal(Math.round(high.rotation.y * 1000), 1571)
  assert.equal(Math.round(high.rotation.p * 1000), 698)
  assert.equal(high.rotation.r, 0)
  assert.equal(high.durationSeconds, 0.75)

  const low = normalizeHeadRequest({ pan_deg: -120, tilt_deg: -40, speed: 0 })
  assert.equal(low.panDeg, -90)
  assert.equal(low.tiltDeg, 5)
  assert.equal(Math.round(low.rotation.y * 1000), -1571)
  assert.equal(Math.round(low.rotation.p * 1000), -698)
  assert.equal(low.durationSeconds, 1.5)
})

test('head request rejects unsafe speed values before servo control', () => {
  assert.throws(() => normalizeHeadRequest({ pan_deg: 0, tilt_deg: 30, speed: -0.1 }), /speed/)
  assert.throws(() => normalizeHeadRequest({ pan_deg: 0, tilt_deg: 30, speed: 1.1 }), /speed/)
})

test('expression request maps MVP names to Stack-chan emotions', () => {
  assert.deepEqual(normalizeExpressionRequest({ emotion: 'happy' }), {
    emotion: 'happy',
    stackchanEmotion: 'HAPPY',
  })
  assert.deepEqual(normalizeExpressionRequest({ emotion: 'surprised' }), {
    emotion: 'surprised',
    stackchanEmotion: 'DOUBTFUL',
  })
  assert.throws(() => normalizeExpressionRequest({ emotion: 'furious' }), /unsupported expression/)
})

test('LED request validates #RRGGBB colors and applies brightness', () => {
  assert.deepEqual(normalizeLEDRequest({ pattern: 'solid', color: '#00AEEF', brightness: 0.5 }), {
    pattern: 'solid',
    color: '#00AEEF',
    brightness: 0.5,
    rgb: { r: 0, g: 87, b: 120 },
  })

  assert.deepEqual(normalizeLEDRequest({ pattern: 'off', color: '#FFFFFF', brightness: 1 }), {
    pattern: 'off',
    color: '#FFFFFF',
    brightness: 1,
    rgb: { r: 0, g: 0, b: 0 },
  })

  assert.throws(() => normalizeLEDRequest({ pattern: 'solid', color: 'blue', brightness: 0.5 }), /invalid LED color/)
  assert.throws(() => normalizeLEDRequest({ pattern: 'rainbow', color: '#00AEEF', brightness: 0.5 }), /unsupported LED pattern/)
})

test('motion request allowlist normalizes names', () => {
  assert.deepEqual(normalizeMotionRequest({ name: 'nod' }), { name: 'nod' })
  assert.deepEqual(normalizeMotionRequest({ name: 'home' }), { name: 'home' })
  assert.throws(() => normalizeMotionRequest({ name: 'dance' }), /unsupported motion/)
})

test('speech request requires bounded text', () => {
  assert.deepEqual(normalizeSpeechRequest({ text: 'hello stack-chan' }), { text: 'hello stack-chan' })
  assert.deepEqual(normalizeSpeechRequest({ text: 'quiet hello', volume: 0.15 }), { text: 'quiet hello', volume: 0.15 })
  assert.throws(() => normalizeSpeechRequest({ text: '' }), /text is required/)
  assert.throws(() => normalizeSpeechRequest({ text: 'x'.repeat(241) }), /text must be 240 characters or fewer/)
  assert.throws(() => normalizeSpeechRequest({ text: 'too loud', volume: 1.1 }), /volume/)
})

test('action response keeps the firmware response shape aligned with the MCP bridge', () => {
  assert.deepEqual(actionResponse('move_head', { tilt_deg: clampTiltDeg(120) }), {
    ok: true,
    action: 'move_head',
    state: { tilt_deg: 85 },
  })
})
