// Render the real shipped Canvas bundle. Fixtures replace network only, not UI code.
import { createCanvas, GlobalFonts } from "@napi-rs/canvas";
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import { scene, deck } from "../tests/scenes.mjs";
GlobalFonts.loadSystemFonts();
// Windows font discovery does not always enumerate TTC fallback families.
if (process.platform === "win32")
  GlobalFonts.registerFromPath("C:/Windows/Fonts/msyh.ttc", "sans-serif");
const output = new URL("../.qa/", import.meta.url);
mkdirSync(output, { recursive: true });
const bundle = readFileSync(
  new URL("../dist/game.js", import.meta.url),
  "utf8",
);
export function render(state, buttons = [], opts = {}) {
  const canvas = createCanvas(960, 540),
    raw = canvas.getContext("2d"),
    events = {},
    drawn = [],
    storage = new Map([["shengji-room", "F8K2Q6"]]);
  let handlers = {};
  const ctx = new Proxy(raw, {
    get: (t, k) =>
      k === "fillText"
        ? (label, x, y, ...rest) => {
            drawn.push({ label, x, y });
            return t.fillText(label, x, y, ...rest);
          }
        : k === "clearRect"
          ? (...a) => {
              drawn.length = 0;
              return t.clearRect(...a);
            }
          : typeof t[k] === "function"
            ? t[k].bind(t)
            : t[k],
    set: (t, k, v) => {
      t[k] = v;
      return true;
    },
  });
  const wx = {
    createCanvas: () =>
      new Proxy(canvas, {
        get: (t, k) =>
          k === "getContext"
            ? () => ctx
            : typeof t[k] === "function"
              ? t[k].bind(t)
              : t[k],
        set: (t, k, v) => {
          t[k] = v;
          return true;
        },
      }),
    getSystemInfoSync: () => ({
      windowWidth: 960,
      windowHeight: 540,
      pixelRatio: 1,
    }),
    getStorageSync: (k) => storage.get(k),
    setStorageSync: (k, v) => storage.set(k, v),
    removeStorageSync: (k) => storage.delete(k),
    showShareMenu() {},
    setClipboardData: (o) => o.success?.(),
    shareAppMessage() {},
    showModal() {},
    connectSocket: () => ({
      onOpen: (f) => (handlers.open = f),
      onMessage: (f) => (handlers.message = f),
      onClose: (f) => (handlers.close = f),
      onError: (f) => (handlers.error = f),
      send() {},
      close() {},
    }),
  };
  for (const name of [
    "TouchStart",
    "TouchMove",
    "TouchEnd",
    "TouchCancel",
    "Show",
    "Hide",
    "WindowResize",
    "ShareAppMessage",
  ])
    wx[`on${name}`] = (f) => (events[name] = f);
  vm.runInNewContext(bundle, {
    wx,
    console,
    setTimeout: () => 1,
    clearTimeout() {},
  });
  const touch = (x, y) => {
    events.TouchStart({ touches: [{ clientX: x, clientY: y }] });
    events.TouchEnd({});
  };
  const button = (label) => {
    const t = drawn.find((t) => t.label === label);
    if (!t) throw Error(`No ${label}`);
    touch(t.x + 5, t.y);
  };
  if (state) {
    button("加入房间");
    handlers.open();
    handlers.message({
      data: JSON.stringify({
        type: "hello",
        name: "春风",
        uid: "qa0",
        token: "fixture",
      }),
    });
    handlers.message({ data: JSON.stringify({ type: "snapshot", state }) });
  }
  for (const label of buttons) button(label);
  if (opts.disconnect) handlers.error({ errMsg: "connection refused" });
  return { canvas, drawn, touch, button };
}
const cases = [
  ["01-lobby", null],
  ["02-seats", scene("lobby")],
  ["03-call", scene("call_trump")],
  ["04-bottom", scene("bottom")],
  ["05-fight", scene("trump_fight")],
  ["06-play", scene()],
  [
    "07-history",
    scene("play_trick", {
      trick: {
        ...scene().trick,
        lastMoves: Array.from({ length: 4 }, (_, seat) => ({
          seat,
          suitClass: "黑桃",
          followMove: { cards: [deck[seat], deck[seat + 54]] },
        })),
      },
    }),
    ["看上一墩"],
  ],
  ["08-settle", scene("round_settle")],
  ["09-winner", scene("game_over", { winnerTeam: 1 })],
  ["10-rules", null, ["规则"]],
  ["11-room", scene(), ["菜单"]],
  ["12-connection", null, ["连接设置"]],
  ["13-disconnect", scene(), [], { disconnect: true }],
  ["14-spectator", scene("play_trick", { mySeat: -1, myHand: null })],
  ["15-record", scene(), ["菜单", "公开记牌"]],
  ["16-observer-settle", scene("round_settle", { mySeat: -1 })],
  ["17-messages", scene(), ["菜单", "消息记录"]],
];
const gallery = createCanvas(1920, Math.ceil(cases.length / 2) * 540),
  g = gallery.getContext("2d");
for (let i = 0; i < cases.length; i++) {
  const [name, state, buttons, opts] = cases[i],
    { canvas } = render(state, buttons, opts);
  writeFileSync(new URL(`${name}.png`, output), canvas.toBuffer("image/png"));
  g.drawImage(canvas, (i % 2) * 960, Math.floor(i / 2) * 540);
}
writeFileSync(new URL("gallery.png", output), gallery.toBuffer("image/png"));
console.log(`Rendered ${cases.length} scenes: ${fileURLToPath(output)}`);
