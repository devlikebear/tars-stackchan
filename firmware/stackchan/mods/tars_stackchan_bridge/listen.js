import HTTPServer from 'embedded:network/http/server'
import Listener from 'embedded:io/socket/listener'
import URL from 'url'

class Request {
  #url
  #method
  #headers
  #body

  constructor(url, options) {
    this.#url = url
    this.#method = options.method
    this.#headers = options.headers
    this.#body = options.body
  }

  get headers() {
    return this.#headers
  }

  get method() {
    return this.#method
  }

  get url() {
    return this.#url
  }

  async json() {
    let body = this.#body
    if (body) {
      this.#body = undefined
      body = await body
      if (body) {
        return JSON.parse(String.fromArrayBuffer(body))
      }
    }
    return body
  }

  async text() {
    let body = this.#body
    if (body) {
      this.#body = undefined
      body = await body
      if (body) {
        return String.fromArrayBuffer(body)
      }
    }
    return body
  }
}

export default async function* listen(options) {
  const connectionQueue = []
  const promiseQueue = []
  const port = options?.port ?? 80
  const base = `http://localhost:${port}`

  const httpServer = new HTTPServer({
    io: Listener,
    port,
    onConnect(rawConnection) {
      let requestBody = null
      let responseBody = null
      let offset = 0
      let length = 0
      let resolveRequest
      let rejectRequest
      let resolveResponse
      let rejectResponse

      const requestPromise = new Promise((resolve, reject) => {
        resolveRequest = resolve
        rejectRequest = reject
      })
      const responsePromise = new Promise((resolve, reject) => {
        resolveResponse = resolve
        rejectResponse = reject
      })

      rawConnection.accept({
        onRequest(rawRequest) {
          const { method, path, headers } = rawRequest
          const request = new Request(new URL(path, base), {
            method,
            path,
            headers,
            body: requestPromise,
          })
          const connection = {
            request,
            close() {
              rawConnection.close()
            },
            async respondWith(response) {
              response = await response
              responseBody = await response.arrayBuffer()
              if (responseBody) {
                length = responseBody.byteLength
              }
              const rawResponse = await responsePromise
              rawResponse.status = response.status
              rawResponse.headers = response.headers
              rawConnection.respond(rawResponse)
            },
          }

          if (promiseQueue.length === 0) {
            connectionQueue.push(connection)
          } else {
            promiseQueue.shift().resolveEvent(connection)
          }
        },
        onReadable(count) {
          if (requestBody) {
            requestBody = requestBody.concat(this.read(count))
          } else {
            requestBody = this.read(count)
          }
        },
        onResponse(rawResponse) {
          resolveRequest(requestBody)
          resolveResponse(rawResponse)
        },
        onWritable(count) {
          if (responseBody) {
            if (length > 0) {
              if (count > length) {
                count = length
              }
              const view = new DataView(responseBody, offset, count)
              this.write(view)
              offset += count
              length -= count
            } else {
              this.write()
            }
          } else {
            this.write()
          }
        },
        onError(error) {
          rejectRequest(error)
          rejectResponse(error)
        },
      })
    },
    onError(message) {
      const error = new Error(message)
      if (promiseQueue.length === 0) {
        connectionQueue.push(error)
      } else {
        promiseQueue.shift().rejectEvent(error)
      }
    },
  })

  while (true) {
    if (!httpServer) {
      throw new Error('HTTP server is not available')
    }
    yield new Promise((resolveEvent, rejectEvent) => {
      if (connectionQueue.length === 0) {
        promiseQueue.push({ resolveEvent, rejectEvent })
      } else {
        const connection = connectionQueue.shift()
        if (connection instanceof Error) {
          rejectEvent(connection)
        } else {
          resolveEvent(connection)
        }
      }
    })
  }
}
