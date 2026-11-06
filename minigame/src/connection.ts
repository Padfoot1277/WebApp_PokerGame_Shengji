import type {
  ServerMessage,
  ClientEvent,
} from "../../frontend/src/types/protocol";

// A failed connection is not necessarily a closable SocketTask in WeChat.
// Invalidate callbacks before scheduling a retry, and only close opened tasks.
export class GameConnection {
  private task: WechatMinigame.SocketTask | null = null;
  private opened = false;
  private epoch = 0;
  private retry = 0;
  private timer: ReturnType<typeof setTimeout> | undefined;
  private ackTimer: ReturnType<typeof setTimeout> | undefined;
  private connectTimer: ReturnType<typeof setTimeout> | undefined;
  private running = false;
  private pending: ClientEvent | null = null;
  status = "未连接";
  server = "";
  room = "";
  name = "";
  constructor(
    private receive: (msg: ServerMessage) => void,
    private change: () => void,
  ) {}
  get busy() {
    return this.pending !== null;
  }
  connect(server: string, room: string, name: string) {
    this.close();
    this.server = server.trim().replace(/\/+$/, "");
    this.room = room;
    this.name = name;
    if (!/^wss?:\/\/[^\s?#]+$/i.test(this.server)) {
      this.status = "地址格式有误，请填写 ws://电脑IP:8080/ws";
      this.change();
      return;
    }
    if (
      /^wss?:\/\/(localhost|127(?:\.\d+){3}|0\.0\.0\.0|\[::1\])(?=[:/]|$)/i.test(
        this.server,
      )
    ) {
      this.status = "微信不能用 localhost，请填写电脑的局域网 IP";
      this.change();
      return;
    }
    this.retry = 0;
    this.running = true;
    this.open();
  }
  private key() {
    return `shengji-token:${this.server}`;
  }
  private clearTimers() {
    clearTimeout(this.timer);
    clearTimeout(this.ackTimer);
    clearTimeout(this.connectTimer);
  }
  private release() {
    const task = this.task;
    const opened = this.opened;
    this.task = null;
    this.opened = false;
    if (task && opened) {
      try {
        task.close({ fail: () => {} });
      } catch {
        /* Already removed by the runtime. */
      }
    }
  }
  private failed(epoch: number, reason: string) {
    if (epoch !== this.epoch || !this.running) return;
    this.epoch++;
    this.clearTimers();
    this.release();
    if (
      /localhost|合法域名|not in domain|url not in|url domain|invalid url/i.test(
        reason,
      )
    ) {
      this.running = false;
      this.pending = null;
      this.status = "微信拒绝此地址，请检查局域网 IP 或 WSS 合法域名";
    } else if (this.retry >= 5) {
      this.running = false;
      this.pending = null;
      this.status = "连接失败：请启动电脑后端并确认同一 Wi-Fi，再点加入房间";
    } else {
      this.status = `无法连接电脑后端，正在重试（${this.retry + 1}/5）`;
      this.timer = setTimeout(
        () => this.open(),
        Math.min(8000, 500 * 2 ** this.retry++),
      );
    }
    this.change();
  }
  private open() {
    if (!this.running) return;
    const epoch = ++this.epoch;
    this.status = "正在连接电脑后端";
    this.change();
    const token = wx.getStorageSync(this.key()) || "";
    const query = `room=${encodeURIComponent(this.room)}&name=${encodeURIComponent(this.name)}${token ? "&token=" + encodeURIComponent(token) : ""}`;
    let task: WechatMinigame.SocketTask;
    try {
      task = wx.connectSocket({ url: this.server + "?" + query });
    } catch {
      this.failed(epoch, "连接创建失败");
      return;
    }
    this.task = task;
    task.onOpen(() => {
      if (epoch === this.epoch && this.running) this.opened = true;
      else {
        // A timed-out or replaced task may open late. It is closable now.
        try {
          task.close({ fail: () => {} });
        } catch {
          /* Removed by wx. */
        }
      }
    });
    task.onMessage((event) => {
      if (epoch !== this.epoch) return;
      let msg: ServerMessage;
      try {
        msg = JSON.parse(String(event.data));
      } catch {
        return;
      }
      if (msg.type === "hello") {
        clearTimeout(this.connectTimer);
        wx.setStorageSync(this.key(), msg.token);
        this.status = "已连接";
        this.retry = 0;
        if (this.pending) this.transmit();
      }
      if (msg.type === "ack") {
        if (msg.reqId !== this.pending?.reqId) return;
        this.pending = null;
        clearTimeout(this.ackTimer);
      }
      this.receive(msg);
      this.change();
    });
    task.onClose((event) => {
      if (epoch !== this.epoch || !this.running) return;
      this.opened = false;
      if (event.code === 4001 || event.code === 4003) {
        this.close();
        if (event.code === 4003) wx.removeStorageSync(this.key());
        this.status =
          event.code === 4001 ? "身份已在其他设备连接" : "身份过期，请重新连接";
        this.change();
        return;
      }
      this.failed(epoch, "连接已断开");
    });
    task.onError((error) => this.failed(epoch, error.errMsg || "网络错误"));
    this.connectTimer = setTimeout(() => this.failed(epoch, "连接超时"), 10000);
  }
  send(type: string, payload: unknown) {
    if (this.status !== "已连接" || this.pending) return false;
    this.pending = {
      type,
      payload,
      reqId: `${Date.now()}-${Math.random().toString(36).slice(2)}-${this.epoch}`,
    };
    this.transmit();
    this.change();
    return true;
  }
  private transmit() {
    if (!this.pending || !this.task) return;
    const epoch = this.epoch;
    this.task.send({
      data: JSON.stringify(this.pending),
      fail: () => this.failed(epoch, "发送失败"),
    });
    if (epoch !== this.epoch) return;
    clearTimeout(this.ackTimer);
    this.ackTimer = setTimeout(() => this.failed(epoch, "操作确认超时"), 8000);
  }
  resume() {
    if (!this.running) return;
    this.epoch++;
    this.clearTimers();
    this.release();
    this.retry = 0;
    this.open();
  }
  retryNow() {
    if (this.running) this.resume();
    else if (this.server) this.connect(this.server, this.room, this.name);
  }
  close() {
    this.running = false;
    this.epoch++;
    this.clearTimers();
    this.release();
    this.pending = null;
    this.status = "未连接";
    this.change();
  }
}
