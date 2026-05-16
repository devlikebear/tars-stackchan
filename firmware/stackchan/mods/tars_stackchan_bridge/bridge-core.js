export const SAFE_TILT_MIN = 5
export const SAFE_TILT_MAX = 85
export const SAFE_PAN_MIN = -90
export const SAFE_PAN_MAX = 90
export const DEFAULT_CAPABILITIES = ['expression', 'head', 'leds', 'motion']

const DEG_TO_RAD = Math.PI / 180
const HEAD_NEUTRAL_TILT_DEG = 45

const EXPRESSION_MAP = Object.freeze({
  angry: 'ANGRY',
  blink: 'NEUTRAL',
  happy: 'HAPPY',
  neutral: 'NEUTRAL',
  sad: 'SAD',
  sleepy: 'SLEEPY',
  surprised: 'DOUBTFUL',
})

const LED_PATTERNS = Object.freeze({
  blink: true,
  off: true,
  pulse: true,
  solid: true,
})

const MOTIONS = Object.freeze({
  home: true,
  look_around: true,
  nod: true,
  shake: true,
})

export function clampTiltDeg(value) {
  return clamp(numberField(value, 'tilt_deg'), SAFE_TILT_MIN, SAFE_TILT_MAX)
}

export function clampPanDeg(value) {
  return clamp(numberField(value, 'pan_deg'), SAFE_PAN_MIN, SAFE_PAN_MAX)
}

export function isAuthorized(authorizationHeader, token) {
  if (typeof token !== 'string' || token.length === 0) {
    throw new Error('TARS Stack-chan token is not configured')
  }
  return authorizationHeader === `Bearer ${token}`
}

export function normalizeExpressionRequest(payload) {
  const request = objectPayload(payload)
  const emotion = stringField(request.emotion, 'emotion')
  const stackchanEmotion = EXPRESSION_MAP[emotion]
  if (!stackchanEmotion) {
    throw new Error(`unsupported expression: ${emotion}`)
  }
  return { emotion, stackchanEmotion }
}

export function normalizeHeadRequest(payload) {
  const request = objectPayload(payload)
  const panDeg = clampPanDeg(request.pan_deg)
  const tiltDeg = clampTiltDeg(request.tilt_deg)
  const speed = numberField(request.speed, 'speed')
  if (speed < 0 || speed > 1) {
    throw new Error('speed must be between 0.0 and 1.0')
  }

  return {
    panDeg,
    tiltDeg,
    speed,
    rotation: {
      y: panDeg * DEG_TO_RAD,
      p: (tiltDeg - HEAD_NEUTRAL_TILT_DEG) * DEG_TO_RAD,
      r: 0,
    },
    durationSeconds: Math.round((1.5 - speed * 1.25) * 100) / 100,
  }
}

export function normalizeLEDRequest(payload) {
  const request = objectPayload(payload)
  const pattern = stringField(request.pattern, 'pattern')
  if (!LED_PATTERNS[pattern]) {
    throw new Error(`unsupported LED pattern: ${pattern}`)
  }

  const color = stringField(request.color, 'color')
  if (!/^#[0-9A-Fa-f]{6}$/.test(color)) {
    throw new Error(`invalid LED color: ${color}`)
  }

  const brightness = numberField(request.brightness, 'brightness')
  if (brightness < 0 || brightness > 1) {
    throw new Error('brightness must be between 0.0 and 1.0')
  }

  return {
    pattern,
    color,
    brightness,
    rgb: pattern === 'off' ? { r: 0, g: 0, b: 0 } : scaleHexColor(color, brightness),
  }
}

export function normalizeMotionRequest(payload) {
  const request = objectPayload(payload)
  const name = stringField(request.name, 'name')
  if (!MOTIONS[name]) {
    throw new Error(`unsupported motion: ${name}`)
  }
  return { name }
}

export function actionResponse(action, state = undefined) {
  return state === undefined ? { ok: true, action } : { ok: true, action, state }
}

function objectPayload(payload) {
  if (payload == null || typeof payload !== 'object' || Array.isArray(payload)) {
    throw new Error('JSON object payload is required')
  }
  return payload
}

function stringField(value, field) {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new Error(`${field} is required`)
  }
  return value
}

function numberField(value, field) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`${field} must be a finite number`)
  }
  return value
}

function scaleHexColor(color, brightness) {
  const r = Number.parseInt(color.slice(1, 3), 16)
  const g = Number.parseInt(color.slice(3, 5), 16)
  const b = Number.parseInt(color.slice(5, 7), 16)
  return {
    r: Math.round(r * brightness),
    g: Math.round(g * brightness),
    b: Math.round(b * brightness),
  }
}

function clamp(value, minValue, maxValue) {
  return Math.min(maxValue, Math.max(minValue, value))
}
