import { URLSearchParams } from 'url'
import listen from 'tars-listen'

class Request {
  raw

  constructor(request) {
    this.raw = request
  }

  get method() {
    return this.raw.method.toLowerCase()
  }

  get path() {
    return this.raw.url.pathname
  }

  header(key) {
    return this.raw.headers.get(key.toLowerCase())
  }

  query(key) {
    const url = this.raw.url
    const params = url?.searchParams
    if (params && typeof params.get === 'function') {
      const value = params.get(key)
      return value === null ? undefined : value
    }
    return undefined
  }

  async text() {
    return await this.raw.text()
  }

  async json() {
    return await this.raw.json()
  }

  async formData() {
    return Object.fromEntries(new URLSearchParams(await this.text()))
  }
}

class Response {
  #body
  #headers
  #status = 200

  constructor(body, options = {}) {
    this.#body = body instanceof ArrayBuffer ? body : ArrayBuffer.fromString((body ?? '').toString())
    let bodyLength = this.#body.byteLength
    if (bodyLength === undefined) {
      bodyLength = this.#body.length
    }
    if (bodyLength === undefined) {
      bodyLength = 0
    }
    const headers = new Map()
    if (options.headers) {
      for (const [key, value] of Object.entries(options.headers)) {
        if (value !== undefined && value !== null) {
          headers.set(key, value.toString())
        }
      }
    }
    if (!headers.has('content-length')) {
      headers.set('content-length', bodyLength.toString())
    }
    this.#headers = headers
    this.#status = options.status ?? 200
  }

  get body() {
    return this.#body
  }

  get headers() {
    return this.#headers
  }

  get status() {
    return this.#status
  }

  async arrayBuffer() {
    let body = this.#body
    if (body) {
      this.#body = undefined
      body = await body
    }
    return body
  }
}

class Context {
  #req
  #status
  #headers = new Map()

  constructor(request) {
    this.#req = new Request(request)
  }

  get req() {
    return this.#req
  }

  text(text, status) {
    this.#headers.set('content-type', 'text/plain')
    return new Response(text, {
      status: status ?? this.#status,
      headers: Object.fromEntries(this.#headers.entries()),
    })
  }

  json(json, status) {
    this.#headers.set('content-type', 'application/json')
    return new Response(JSON.stringify(json), {
      status: status ?? this.#status,
      headers: Object.fromEntries(this.#headers.entries()),
    })
  }

  body(buffer, contentType, status) {
    this.#headers.set('content-type', contentType)
    return new Response(buffer, {
      status: status ?? this.#status,
      headers: Object.fromEntries(this.#headers.entries()),
    })
  }
}

class HttpServerService {
  #routes = {
    get: new Map(),
    post: new Map(),
  }
  #listener
  #task

  get = (path, handler) => this.#routes.get.set(path, handler)
  post = (path, handler) => this.#routes.post.set(path, handler)

  constructor(options = {}) {
    this.#listener = listen({ port: options?.port })
    this.#task = this.#serve()
  }

  async #serve() {
    try {
      for await (const connection of this.#listener) {
        const context = new Context(connection.request)
        const req = context.req
        let response
        try {
          const handler = this.#routes[req.method]?.get(req.path)
          response = handler ? await handler(context) : context.text('Resource Not Found', 404)
        } catch (error) {
          trace(`[tars-stackchan] HTTP handler failed: ${error?.message ?? error}\n`)
          response = context.text('Internal Server Error', 500)
        }
        await connection.respondWith(response)
      }
    } catch (error) {
      trace(`[tars-stackchan] HTTP server failed: ${error?.message ?? error}\n`)
    }
  }
}

export { HttpServerService, Response }
