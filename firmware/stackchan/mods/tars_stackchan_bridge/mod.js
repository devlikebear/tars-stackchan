import config from 'mc/config'
import Net from 'net'
import { HttpServerService } from 'http-server-service'

import {
  DEFAULT_CAPABILITIES,
  actionResponse,
  isAuthorized,
  normalizeExpressionRequest,
  normalizeHeadRequest,
  normalizeLEDRequest,
  normalizeMotionRequest,
} from './bridge-core'

const bridgeConfig = config.tarsStackchan ?? {}
const TOKEN = bridgeConfig.token ?? ''
const DEVICE = bridgeConfig.device ?? 'stackchan-k151'
const FIRMWARE = bridgeConfig.firmware ?? 'tars-stackchan-dev'
const LED_NAME = bridgeConfig.ledName ?? 'a'
const PORT = bridgeConfig.port

const FORWARD = Object.freeze({ y: 0, p: 0, r: 0 })
const MOTION_STEPS = Object.freeze({
  home: [FORWARD],
  nod: [
    { y: 0, p: -0.35, r: 0 },
    { y: 0, p: 0.35, r: 0 },
    FORWARD,
  ],
  shake: [
    { y: -0.35, p: 0, r: 0 },
    { y: 0.35, p: 0, r: 0 },
    FORWARD,
  ],
  look_around: [
    { y: -0.45, p: -0.1, r: 0 },
    { y: 0.45, p: 0.1, r: 0 },
    FORWARD,
  ],
})

function onRobotCreated(robot) {
  const server = new HttpServerService({ port: PORT })

  server.get('/v1/status', (c) =>
    c.json({
      connected: true,
      device: DEVICE,
      firmware: FIRMWARE,
      battery_percent: undefined,
      ip: getIP(),
      capabilities: DEFAULT_CAPABILITIES,
    }),
  )

  server.post('/v1/expression', withAuth(async (c) => {
    const request = normalizeExpressionRequest(await readJSON(c))
    robot.setEmotion(request.stackchanEmotion)
    return c.json(actionResponse('set_expression', { emotion: request.emotion }))
  }))

  server.post('/v1/head', withAuth(async (c) => {
    const request = normalizeHeadRequest(await readJSON(c))
    await robot.driver.setTorque(true)
    await robot.driver.applyRotation(request.rotation, request.durationSeconds)
    return c.json(
      actionResponse('move_head', {
        pan_deg: request.panDeg,
        tilt_deg: request.tiltDeg,
        speed: request.speed,
      }),
    )
  }))

  server.post('/v1/leds', withAuth(async (c) => {
    const request = normalizeLEDRequest(await readJSON(c))
    const { r, g, b } = request.rgb
    switch (request.pattern) {
      case 'off':
        robot.lightOff(LED_NAME)
        break
      case 'blink':
        robot.lightBlink(LED_NAME, r, g, b, 500)
        break
      case 'pulse':
        robot.lightBlink(LED_NAME, r, g, b, 1200)
        break
      default:
        robot.lightOn(LED_NAME, r, g, b)
    }
    return c.json(
      actionResponse('set_led', {
        pattern: request.pattern,
        color: request.color,
        brightness: request.brightness,
      }),
    )
  }))

  server.post('/v1/motion', withAuth(async (c) => {
    const request = normalizeMotionRequest(await readJSON(c))
    await runMotion(robot, request.name)
    return c.json(actionResponse('run_motion', { name: request.name }))
  }))

  trace(`[tars-stackchan] local control API listening${PORT ? ` on port ${PORT}` : ''}\n`)
}

function withAuth(handler) {
  return async (c) => {
    try {
      if (!isAuthorized(c.req.header('authorization'), TOKEN)) {
        return c.json({ error: 'invalid token' }, 401)
      }
      return await handler(c)
    } catch (error) {
      const message = error?.message ?? String(error)
      trace(`[tars-stackchan] request failed: ${message}\n`)
      return c.json({ error: message }, message.includes('token is not configured') ? 503 : 400)
    }
  }
}

async function readJSON(c) {
  try {
    return await c.req.json()
  } catch (_error) {
    throw new Error('invalid JSON body')
  }
}

async function runMotion(robot, name) {
  const steps = MOTION_STEPS[name]
  await robot.driver.setTorque(true)
  for (const rotation of steps) {
    await robot.driver.applyRotation(rotation, 0.35)
  }
}

function getIP() {
  try {
    return Net.get('IP')
  } catch (_error) {
    return undefined
  }
}

export default {
  onRobotCreated,
}
