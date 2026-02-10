import { defineStore } from 'pinia'
import { wsService } from '../services/ws'
import type { ServerMessage } from '../types/protocol'

type MessageItem = {
    id: number
    level: 'error' | 'notice'
    text: string
}

export const useGameStore = defineStore('game', {
    state: () => ({
        uid: null as string | null,      // 来自 hello
        view: null as any,               // ViewState（下一步再强类型）
        connected: false,

        messages: [] as MessageItem[],
        _msgId: 0,
        wsStatus: 'idle' as 'idle' | 'connecting' | 'open' | 'closed',

    }),

    actions: {
        connect(wsUrl: string) {
            wsService.connect(wsUrl, this.handleServerMessage, (s) => {
                this.wsStatus = s
            })
        },

        handleServerMessage(msg: ServerMessage) {
            console.log('[WS MESSAGE]', msg)
            switch (msg.type) {
                case 'hello':
                    this.uid = msg.uid
                    break

                case 'snapshot':
                    // authoritative：整包替换
                    this.view = msg.state
                    break

                case 'error':
                    this.pushMessage('error', msg.message)
                    break

                case 'notice':
                    this.pushMessage('notice', msg.message)
                    break

                default:
                    console.warn('[store] unknown message', msg)
            }
        },

        sendEvent<T>(type: string, payload: T) {
            wsService.send(type, payload)
        },

        pushMessage(level: 'error' | 'notice', text: string) {
            this.messages.push({
                id: ++this._msgId,
                level,
                text,
            })

            // 只保留最近 50 条（UI 默认展示 5）
            if (this.messages.length > 50) {
                this.messages.shift()
            }
        },

        disconnect() {
            // 需要 wsService.close() 已实现
            // 断开后等待用户再次 connect
            // 可选：是否清空 view 由你决定（建议不清空，让用户还能看到上一帧）
            wsService.close()
            this.wsStatus = 'closed'
        },
    },
})
