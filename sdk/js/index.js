export class LyricSyncClient {
  constructor(baseURL = 'http://127.0.0.1:41917', options = {}) {
    this.baseURL = String(baseURL).replace(/\/$/, '')
    this.fetch = options.fetch || globalThis.fetch
    if (!this.fetch) {
      throw new Error('LyricSyncClient requires fetch')
    }
  }

  health() { return this.get('/api/health') }
  state() { return this.get('/api/state') }
  song() { return this.get('/api/song') }
  lyric() { return this.get('/api/lyric') }
  sessions() { return this.get('/api/session') }
  clients() { return this.get('/api/clients') }
  logs() { return this.get('/api/logs') }
  config() { return this.get('/api/config') }
  metrics() { return this.get('/api/metrics') }

  async get(path) {
    const response = await this.fetch(`${this.baseURL}${path}`)
    if (!response.ok) {
      throw new Error(`LyricSync GET ${path} failed: ${response.status}`)
    }
    return response.json()
  }

  eventWebSocketURL() {
    return this.webSocketURL('/ws')
  }

  amllWebSocketURL() {
    return this.webSocketURL('/amll/ws')
  }

  connectEvents(WebSocketImpl = globalThis.WebSocket) {
    if (!WebSocketImpl) throw new Error('WebSocket implementation is required')
    return new WebSocketImpl(this.eventWebSocketURL())
  }

  connectAMLL(WebSocketImpl = globalThis.WebSocket) {
    if (!WebSocketImpl) throw new Error('WebSocket implementation is required')
    return new WebSocketImpl(this.amllWebSocketURL())
  }

  webSocketURL(path) {
    const url = new URL(this.baseURL)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    url.pathname = path
    url.search = ''
    return url.toString()
  }
}
