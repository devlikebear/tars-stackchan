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
    "speech"
  ]
}
```

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
```

Future endpoints:

```text
POST /v1/ir/send
GET  /v1/sensors
GET  /v1/camera/snapshot
```
