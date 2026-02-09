import type { ServerMessage, ClientEvent } from '../types/protocol'

type MessageHandler = (msg: ServerMessage) => void
type StatusHandler = (s: 'idle' | 'connecting' | 'open' | 'closed') => void

class WSService {
    private ws: WebSocket | null = null
    private url = ''
    private handler: MessageHandler | null = null
    private onStatus: StatusHandler | null = null
    private manualClose = false
    private retry = 0
    private retryTimer: number | null = null

    connect(url: string, handler: MessageHandler, onStatus?: StatusHandler) {
        this.url = url
        this.handler = handler
        this.onStatus = onStatus ?? null
        this.manualClose = false
        this.retry = 0
        this.open()
    }

    private open() {
        if (!this.url || !this.handler) return
        this.onStatus?.('connecting')

        if (this.ws) {
            try { this.ws.close() } catch {}
            this.ws = null
        }

        this.ws = new WebSocket(this.url)

        this.ws.onopen = () => {
            this.retry = 0
            this.onStatus?.('open')
            console.log('[WS] connected')
        }

        this.ws.onmessage = (ev) => {
            try {
                const msg = JSON.parse(ev.data) as ServerMessage
                this.handler?.(msg)
            } catch {
                console.warn('[WS] invalid message', ev.data)
            }
        }

        this.ws.onclose = () => {
            this.onStatus?.('closed')
            console.warn('[WS] closed')
            if (!this.manualClose) this.scheduleReconnect()
        }

        this.ws.onerror = (err) => {
            console.error('[WS] error', err)
        }
    }

    private scheduleReconnect() {
        if (this.retryTimer) window.clearTimeout(this.retryTimer)
        const delay = Math.min(8000, 500 * Math.pow(2, this.retry)) // 0.5s,1s,2s,4s,8s
        this.retry = Math.min(this.retry + 1, 5)
        this.retryTimer = window.setTimeout(() => this.open(), delay)
    }

    send<T>(type: string, payload: T) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            console.warn('[WS] send failed, not open')
            return
        }
        const evt: ClientEvent<T> = { type, payload }
        this.ws.send(JSON.stringify(evt))
    }

    close() {
        this.manualClose = true
        if (this.retryTimer) window.clearTimeout(this.retryTimer)
        this.retryTimer = null
        this.ws?.close()
        this.ws = null
        this.onStatus?.('closed')
    }
}

export const wsService = new WSService()
