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
  const canvas = createCanvas(390, 844),
    raw = canvas.getContext("2d"),
    events = {},
    drawn = [],
    storage = new Map([["shengji-room", "F8K2Q6"]]);
  let handlers = {},
    now = 10000,
    timerId = 0;
  const timers = new Map();
  class Clock extends Date {
    static now() {
      return now;
    }
  }
  const advance = (ms) => {
    const end = now + ms;
    while (true) {
      const next = [...timers]
        .filter(([, t]) => t.at <= end)
        .sort((a, b) => a[1].at - b[1].at)[0];
      if (!next) break;
      timers.delete(next[0]);
      now = next[1].at;
      next[1].run();
    }
    now = end;
  };
  let transform = { a: 1, d: 1, e: 0, f: 0 };
  const ctx = new Proxy(raw, {
    get: (t, k) =>
      k === "setTransform"
        ? (a, b, c, d, e, f) => {
            transform = { a, d, e, f };
            return raw.setTransform(a, b, c, d, e, f);
          }
        : k === "fillText"
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
      windowWidth: 390,
      windowHeight: 844,
      pixelRatio: 1,
      ...opts.info,
    }),
    getMenuButtonBoundingClientRect: () => opts.capsule,
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
    Date: Clock,
    setTimeout: (run, ms) => {
      timers.set(++timerId, { run, at: now + ms });
      return timerId;
    },
    clearTimeout: (id) => timers.delete(id),
  });
  const touch = (x, y) => {
    events.TouchStart({
      touches: [
        {
          clientX:
            (x * transform.a + transform.e) / (opts.info?.pixelRatio || 1),
          clientY:
            (y * transform.d + transform.f) / (opts.info?.pixelRatio || 1),
        },
      ],
    });
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
    if (opts.from)
      handlers.message({
        data: JSON.stringify({ type: "snapshot", state: opts.from }),
      });
    handlers.message({ data: JSON.stringify({ type: "snapshot", state }) });
  }
  for (const label of buttons) button(label);
  if (opts.touch) touch(...opts.touch);
  if (opts.elapsed) advance(opts.elapsed);
  if (opts.disconnect) handlers.error({ errMsg: "connection refused" });
  return { canvas, drawn, touch, button, advance };
}
const publicMove = (seat, id) => ({
  seat,
  suitClass: "黑桃",
  followMove: { cards: [deck[id], deck[id + 54]] },
});
const completeTrick = scene("play_trick", {
  version: 3,
  trickIndex: 7,
  points: 105,
  trick: {
    ...scene().trick,
    resolved: true,
    winnerSeat: 1,
    turnSeat: 1,
    lastMoves: [
      publicMove(0, 1),
      publicMove(1, 4),
      publicMove(2, 7),
      publicMove(3, 10),
    ],
  },
});
const upgrade = scene("round_settle", {
  version: 4,
  callerDelta: 2,
  defenderDelta: 0,
  roundPointsFinal: 15,
  bottomAward: 0,
  roundResultLabel: "小光",
});
const freshDeal = scene("call_trump", {
  version: 2,
  points: 0,
  usedTrumpIds: [],
  callPassedSeats: [false, false, false, false],
});
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
  ["18-preferences", null, ["声音与动效"]],
  ["19-deal", freshDeal, [], { from: scene("lobby"), elapsed: 100 }],
  [
    "20-confirmed-move",
    scene("play_trick", {
      version: 2,
      trick: {
        ...scene().trick,
        turnSeat: 1,
        playedMoves: [publicMove(0, 0), null, null, null],
      },
    }),
    [],
    { from: scene(), elapsed: 70 },
  ],
  [
    "21-my-turn",
    scene("play_trick", { version: 2 }),
    [],
    {
      from: scene("play_trick", { trick: { ...scene().trick, turnSeat: 1 } }),
      elapsed: 200,
    },
  ],
  ["22-trick-result", completeTrick, [], { from: scene(), elapsed: 200 }],
  ["23-upgrade", upgrade, [], { from: scene(), elapsed: 150 }],
  ["24-selected", scene(), [], { touch: [40, 480], elapsed: 50 }],
];
const gallery = createCanvas(780, Math.ceil(cases.length / 2) * 844),
  g = gallery.getContext("2d");
for (let i = 0; i < cases.length; i++) {
  const [name, state, buttons, opts] = cases[i],
    { canvas } = render(state, buttons, opts);
  writeFileSync(new URL(`${name}.png`, output), canvas.toBuffer("image/png"));
  g.drawImage(canvas, (i % 2) * 390, Math.floor(i / 2) * 844);
}
writeFileSync(new URL("gallery.png", output), gallery.toBuffer("image/png"));
// Frames at different times prove that effects finish; static fixture snapshots do not.
for (const [name, state, opts] of [
  ["deal", freshDeal, { from: scene("lobby") }],
  ["upgrade", upgrade, { from: scene() }],
]) {
  const strip = createCanvas(780, 1688),
    g = strip.getContext("2d");
  [0, 100, 300, 900].forEach((elapsed, i) => {
    g.drawImage(
      render(state, [], { ...opts, elapsed }).canvas,
      (i % 2) * 390,
      Math.floor(i / 2) * 844,
    );
  });
  writeFileSync(
    new URL(`${name}-frames.png`, output),
    strip.toBuffer("image/png"),
  );
}
console.log(`Rendered ${cases.length} scenes: ${fileURLToPath(output)}`);

for (const [name, info, capsule] of [
  [
    "portrait-small",
    {
      windowWidth: 320,
      windowHeight: 568,
      pixelRatio: 1,
      safeArea: { left: 0, right: 320, top: 20, bottom: 548 },
    },
    { left: 220, right: 310, top: 24, bottom: 54 },
  ],
  [
    "portrait-notch",
    {
      windowWidth: 390,
      windowHeight: 844,
      pixelRatio: 1,
      safeArea: { left: 0, right: 390, top: 44, bottom: 810 },
    },
    { left: 280, right: 380, top: 48, bottom: 80 },
  ],
  [
    "portrait-tablet",
    {
      windowWidth: 768,
      windowHeight: 1024,
      pixelRatio: 1,
      safeArea: { left: 0, right: 768, top: 24, bottom: 1004 },
    },
    null,
  ],
]) {
  const { canvas } = render(scene("bottom"), [], { info, capsule });
  writeFileSync(new URL(`${name}.png`, output), canvas.toBuffer("image/png"));
}
