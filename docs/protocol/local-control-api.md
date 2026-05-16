# Local Control API

This document defines the local HTTP API shared by the `tars-stackchan` MCP server and the Stack-chan firmware bridge.

The API is versioned as `/v1` so MCP clients and firmware can evolve without changing the initial tool contract.

## Transport

- Base URL: configured by `TARS_STACKCHAN_BASE_URL`
- Development default example: `http://stackchan.local`
- Payload format: JSON
- Response format: JSON

## Authentication

Every mutating request must include a bearer token:

```http
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
```

`GET /v1/status` does not require the token so tools can diagnose whether a device is reachable before attempting control.

## Status

```http
GET /v1/status
```

Response:

```json
{
  "connected": true,
  "device": "stackchan-k151",
  "firmware": "tars-stackchan-dev",
  "battery_percent": 87,
  "ip": "192.168.1.42",
  "capabilities": [
    "expression",
    "head",
    "leds",
    "motion",
    "speech",
    "camera",
    "microphone"
  ]
}
```

`camera` and `microphone` appear in `capabilities` only when the firmware
build includes the perception modules and the hardware exposes them (M5Stack
CoreS3). Clients must treat them as optional and degrade gracefully when
absent.

## Expression

```http
POST /v1/expression
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
Content-Type: application/json
```

Request:

```json
{
  "emotion": "happy"
}
```

Allowed expressions:

- `angry`
- `blink`
- `happy`
- `neutral`
- `sad`
- `sleepy`
- `surprised`

## Head

```http
POST /v1/head
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
Content-Type: application/json
```

Request:

```json
{
  "pan_deg": 45,
  "tilt_deg": 30,
  "speed": 0.6
}
```

Safety contract:

- MCP server accepts `pan_deg` as a horizontal angle. Firmware may clamp or normalize it to its actual safe range.
- The firmware overlay clamps `pan_deg` to `-90..90`.
- MCP server clamps `tilt_deg` to `5..85`.
- Firmware must also clamp `tilt_deg` to `5..85`.
- `speed` is normalized to `0.0..1.0`.

## LEDs

```http
POST /v1/leds
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
Content-Type: application/json
```

Request:

```json
{
  "pattern": "solid",
  "color": "#00AEEF",
  "brightness": 0.5
}
```

Allowed LED patterns:

- `blink`
- `off`
- `pulse`
- `solid`

`color` must be `#RRGGBB`. `brightness` is normalized to `0.0..1.0`.

## Motion

```http
POST /v1/motion
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
Content-Type: application/json
```

Request:

```json
{
  "name": "nod"
}
```

Allowed motions:

- `home`
- `look_around`
- `nod`
- `shake`

## Speech

```http
POST /v1/speech
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
Content-Type: application/json
```

Request:

```json
{
  "text": "hello stack-chan",
  "volume": 0.15
}
```

`text` is required and must be 240 characters or fewer. `volume` is optional and normalized to `0.0..1.0`; when omitted, firmware uses Stack-chan's configured TTS volume. Firmware sends both values through Stack-chan's configured `robot.say(...)` speech voice.

## Camera Snapshot

```http
GET /v1/camera/snapshot?max_width=320
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
```

Captures a single still frame from the CoreS3 camera.

- Response: `200` with `Content-Type: image/jpeg` and the JPEG bytes as the body.
- `max_width` is optional. Firmware may clamp it to a supported framesize
  (capture defaults to QVGA 320x240). Firmware is not required to honor
  arbitrary sizes; it picks the nearest supported framesize ≤ `max_width`.
- The firmware bounds resolution and JPEG quality (manifest config) so a
  snapshot stays small enough for the local network and the cloud vision call.
- Requires the camera capability. If the build/hardware lacks a camera, return
  a non-2xx with `{ "error": "camera unavailable" }`.

## Audio Clip

```http
GET /v1/audio/clip?ms=1500
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
```

Records a short clip from the microphone and returns it.

- Response: `200` with `Content-Type: audio/wav` and a PCM WAV body
  (16 kHz, 16-bit, mono — matches the firmware `audioIn` config).
- `ms` is optional (default `1500`). Firmware clamps it to a safe maximum
  (recommended `≤ 3000`) to bound memory and bandwidth.
- One recording at a time. If a capture is already in progress, return a
  non-2xx with `{ "error": "microphone busy" }`.
- Requires the microphone capability.

## Sensors

```http
GET /v1/sensors
Authorization: Bearer <TARS_STACKCHAN_TOKEN>
```

Lightweight state for the perception loop's event trigger. Cheap to poll
(e.g. ~1 Hz); does not capture media.

Response:

```json
{
  "motion": false,
  "sound_level": 0.07,
  "ts": 1747396800000
}
```

- `motion` (bool): firmware-side movement hint when available (e.g. derived
  from the IMU / scene change); `false` when not determinable.
- `sound_level` (float, `0.0..1.0`): normalized recent input level.
- `ts` (int): firmware clock in unix milliseconds.

## Action Response

Mutating endpoints return an action result:

```json
{
  "ok": true,
  "action": "move_head"
}
```

Firmware may include extra state in `state` for diagnostics:

```json
{
  "ok": true,
  "action": "move_head",
  "state": {
    "pan_deg": 45,
    "tilt_deg": 85,
    "speed": 0.6
  }
}
```

## Error Response

Firmware should return a non-2xx HTTP status for rejected requests.

Recommended body:

```json
{
  "error": "invalid token"
}
```

The MCP HTTP bridge surfaces the HTTP status code and response body as the MCP tool error.

## Current Endpoints

```text
GET  /v1/status
POST /v1/expression
POST /v1/head
POST /v1/leds
POST /v1/motion
POST /v1/speech
GET  /v1/camera/snapshot
GET  /v1/audio/clip
GET  /v1/sensors
```

Future endpoints:

```text
POST /v1/ir/send
```
