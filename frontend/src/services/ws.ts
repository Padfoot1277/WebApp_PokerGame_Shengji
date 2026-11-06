import type { ServerMessage, ClientEvent } from "../types/protocol";
type Status = "idle" | "connecting" | "open" | "closed";
class WSService {
  private ws: WebSocket | null = null;
  private url = "";
  private generation = 0;
  private retry = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private handler: ((msg: ServerMessage) => void) | null = null;
  private status: ((s: Status) => void) | null = null;
  private ackTimer: ReturnType<typeof setTimeout> | null = null;
  private pending = new Map<string, ClientEvent>();
  connect(
    url: string,
    handler: (msg: ServerMessage) => void,
    status?: (s: Status) => void,
  ) {
    this.close();
    this.url = url;
    this.handler = handler;
    this.status = status ?? null;
    this.retry = 0;
    this.open(this.generation);
  }
  private open(generation: number) {
    if (generation !== this.generation) return;
    this.status?.("connecting");
    const url = new URL(this.url, location.href);
    const token = sessionStorage.getItem("shengji-token");
    if (token) url.searchParams.set("token", token);
    const ws = new WebSocket(url);
    this.ws = ws;
    ws.onopen = () => {
      if (generation !== this.generation || this.ws !== ws) return;
    };
    ws.onmessage = (event) => {
      if (generation !== this.generation || this.ws !== ws) return;
      let msg: ServerMessage;
      try {
        msg = JSON.parse(event.data);
      } catch {
        return;
      }
      if (msg.type === "hello") {
        this.retry = 0;
        this.status?.("open");
        sessionStorage.setItem("shengji-token", msg.token);
        // Resend only commands from this connection session, with their original IDs.
        for (const request of this.pending.values())
          ws.send(JSON.stringify(request));
        if (this.pending.size) this.armAckTimer();
      }
      if (msg.type === "ack") {
        if (!this.pending.delete(msg.reqId)) return;
        if (this.ackTimer) clearTimeout(this.ackTimer);
        this.ackTimer = null;
      }
      this.handler?.(msg);
    };
    ws.onclose = (event) => {
      if (generation !== this.generation || this.ws !== ws) return;
      if (event.code === 4001 || event.code === 4003) {
        sessionStorage.removeItem("shengji-url");
        this.close();
        this.handler?.({
          type: "error",
          message:
            event.code === 4001
              ? "此身份已在另一处连接，请勿重复打开同一身份"
              : "身份已过期，请在连接设置中退出身份后重新加入",
        });
        return;
      }
      this.status?.("closed");
      this.timer = setTimeout(
        () => this.open(generation),
        Math.min(8000, 500 * 2 ** Math.min(this.retry++, 4)),
      );
    };
    ws.onerror = () => ws.close();
  }
  send<T>(type: string, payload: T): boolean {
    if (this.ws?.readyState !== WebSocket.OPEN || this.pending.size > 0)
      return false;
    const reqId = Array.from(crypto.getRandomValues(new Uint32Array(4)), (n) =>
      n.toString(16),
    ).join("-");
    const event = { type, payload, reqId };
    this.pending.set(reqId, event);
    this.ws.send(JSON.stringify(event));
    this.armAckTimer();
    return true;
  }
  private armAckTimer() {
    if (this.ackTimer) clearTimeout(this.ackTimer);
    this.ackTimer = setTimeout(() => {
      if (this.pending.size) this.ws?.close();
    }, 8000);
  }
  close() {
    if (this.ackTimer) clearTimeout(this.ackTimer);
    this.ackTimer = null;
    this.generation++;
    if (this.timer) clearTimeout(this.timer);
    this.timer = null;
    this.ws?.close();
    this.ws = null;
    this.pending.clear();
    this.status?.("idle");
  }
}
export const wsService = new WSService();
