import type { ViewState } from "../types/view";
import { defineStore } from "pinia";
import { wsService } from "../services/ws";
import type { ServerMessage } from "../types/protocol";

type MessageItem = {
  id: number;
  level: "error" | "notice";
  text: string;
};

export const useGameStore = defineStore("game", {
  state: () => ({
    uid: null as string | null, // 来自 hello
    view: null as ViewState | null, // ViewState（下一步再强类型）
    pending: false,
    reviewPrevious: false,

    messages: [] as MessageItem[],
    _msgId: 0,
    wsStatus: "idle" as "idle" | "connecting" | "open" | "closed",
  }),

  actions: {
    connect(wsUrl: string) {
      sessionStorage.setItem("shengji-url", wsUrl);
      this.view = null;
      this.reviewPrevious = false;
      this.pending = false;
      this.messages = [];
      wsService.connect(wsUrl, this.handleServerMessage, (s) => {
        this.wsStatus = s;
      });
    },

    handleServerMessage(msg: ServerMessage) {
      // console.log('[WS MESSAGE]', msg)  // Debug专用
      switch (msg.type) {
        case "ack":
          this.pending = false;
          if (!msg.ok) this.pushMessage("error", msg.message);
          break;
        case "hello":
          this.uid = msg.uid;
          sessionStorage.setItem("shengji-nickname", msg.name);
          break;

        case "snapshot":
          // authoritative：整包替换
          if (
            this.view?.roomId !== msg.state.roomId ||
            this.view?.roundIndex !== msg.state.roundIndex ||
            this.view?.phase !== msg.state.phase ||
            this.view?.trickIndex !== msg.state.trickIndex ||
            !msg.state.trick.lastMoves.some(Boolean)
          )
            this.reviewPrevious = false;
          this.view = msg.state;
          break;

        case "error":
          this.pending = false;
          this.pushMessage("error", msg.message);
          break;

        case "notice":
          this.pushMessage("notice", msg.message);
          break;

        default:
          console.warn("[store] unknown message", msg);
      }
    },

    sendEvent<T>(type: string, payload: T) {
      if (
        this.reviewPrevious &&
        [
          "game.play_cards",
          "game.put_bottom",
          "game.call_trump",
          "game.change_trump",
          "game.attack_trump",
          "game.call_pass",
        ].includes(type)
      ) {
        this.pushMessage("error", "正在查看上一墩，请先回到当前牌桌再操作");
        return;
      }
      if (!wsService.send(type, payload)) {
        this.pushMessage(
          "error",
          this.pending ? "上一操作正在确认中" : "连接尚未恢复，请稍后重试",
        );
        return;
      }
      this.pending = true;
    },

    pushMessage(level: "error" | "notice", text: string) {
      this.messages.push({
        id: ++this._msgId,
        level,
        text,
      });

      // 只保留最近 50 条（UI 默认展示 5）
      if (this.messages.length > 50) {
        this.messages.shift();
      }
    },

    disconnect() {
      this.reviewPrevious = false;
      sessionStorage.removeItem("shengji-url");
      // 需要 wsService.close()
      // 关闭当前连接
      // 断开后等待用户再次 connect
      // 可选：是否清空 view 由你决定（建议不清空，让用户还能看到上一帧）
      wsService.close();
      this.pending = false;
      this.wsStatus = "idle";
    },
  },
});
