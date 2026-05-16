// Native perception capture, isolated from the pure request logic in
// bridge-core.js so that module stays unit-testable under plain Node.
//
// Camera: ECMA-419 embedded:io/image/in/camera (host module, added by
//   patch 0002). Modeled on $(MODDABLE)/examples/io/imagein/camera/
//   camera-server-jpeg/main.js.
// Microphone: ECMA-419 embedded:io/audio/in (host module, added by
//   patch 0002). WAV framing modeled on upstream stackchan/microphone.ts
//   (16 kHz / 16-bit, matches the host audioIn config and the protocol).
import Camera from 'embedded:io/image/in/camera'
import AudioIn from 'embedded:io/audio/in'

const WAV_HEADER_SIZE = 44

// Capture a single JPEG still. The camera frame is "buffer/disposable", so
// the bytes are copied into a standalone ArrayBuffer before the frame and
// camera are released.
export function captureJpeg(options = {}) {
  const maxWidth = options.maxWidth ?? 320
  return new Promise((resolve, reject) => {
    let settled = false
    let camera
    const finish = (fn, arg) => {
      try {
        camera?.stop?.()
        camera?.close?.()
      } catch (_error) {
        // ignore teardown errors; the result is already decided
      }
      fn(arg)
    }
    try {
      camera = new Camera({
        width: maxWidth,
        imageType: 'jpeg',
        format: 'buffer/disposable',
        onReadable() {
          let frame
          try {
            frame = camera.read()
            if (settled) {
              frame?.close?.()
              return
            }
            settled = true
            const length = frame.byteLength
            const copy = new Uint8Array(length)
            copy.set(new Uint8Array(frame, 0, length))
            frame?.close?.()
            finish(resolve, copy.buffer)
          } catch (error) {
            frame?.close?.()
            if (!settled) {
              settled = true
              finish(reject, error)
            }
          }
        },
      })
      camera.start()
    } catch (error) {
      if (!settled) {
        settled = true
        finish(reject, error)
      }
    }
  })
}

// Record a fixed-duration mono PCM WAV clip.
export function recordWavClip(durationMs = 1500) {
  return new Promise((resolve, reject) => {
    let audioin
    let wavBuffer
    let dataView
    let writeOffset = 0
    let settled = false
    try {
      audioin = new AudioIn({
        onReadable(size) {
          if (settled) {
            return
          }
          try {
            const remaining = dataView.byteLength - writeOffset
            const chunkSize = Math.min(size, remaining)
            const chunk = this.read(chunkSize)
            if (!chunk) {
              settled = true
              this.close()
              resolve(wavBuffer)
              return
            }
            dataView.set(new Uint8Array(chunk), writeOffset)
            writeOffset += chunkSize
            if (writeOffset >= dataView.byteLength) {
              settled = true
              this.close()
              resolve(wavBuffer)
            }
          } catch (error) {
            if (!settled) {
              settled = true
              try {
                this.close()
              } catch (_closeError) {
                // ignore
              }
              reject(error)
            }
          }
        },
      })

      const { sampleRate, channels, bitsPerSample } = audioin
      const byteRate = sampleRate * channels * (bitsPerSample >> 3)
      const contentLength = Math.floor((durationMs / 1000) * byteRate)
      wavBuffer = new ArrayBuffer(WAV_HEADER_SIZE + contentLength)
      dataView = new Uint8Array(wavBuffer, WAV_HEADER_SIZE)
      writeWavHeader(new DataView(wavBuffer), contentLength, channels, sampleRate, byteRate, bitsPerSample)

      audioin.start()
    } catch (error) {
      if (!settled) {
        settled = true
        reject(error)
      }
    }
  })
}

function writeWavHeader(view, contentLength, channels, sampleRate, byteRate, bitsPerSample) {
  writeAscii(view, 0, 'RIFF')
  view.setUint32(4, 36 + contentLength, true)
  writeAscii(view, 8, 'WAVE')
  writeAscii(view, 12, 'fmt ')
  view.setUint32(16, 16, true)
  view.setUint16(20, 1, true) // PCM
  view.setUint16(22, channels, true)
  view.setUint32(24, sampleRate, true)
  view.setUint32(28, byteRate, true)
  view.setUint16(32, (channels * bitsPerSample) >> 3, true)
  view.setUint16(34, bitsPerSample, true)
  writeAscii(view, 36, 'data')
  view.setUint32(40, contentLength, true)
}

function writeAscii(view, offset, text) {
  for (let i = 0; i < text.length; i += 1) {
    view.setUint8(offset + i, text.charCodeAt(i))
  }
}
