import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
function boot(options = {}) {
  const events = {};
  const texts = [];
  const storage = new Map(options.storage ?? []);
  const rendered = [],
    modals = [],
    clipboard = [],
    shares = [];
  const tasks = [];
  let tid = 0;
  let clock = 1_700_000_000_000;
  const timers = new Map(),
    audioContexts = [],
    audioOptions = [],
    vibrations = [];
  class FakeDate extends Date {
    constructor(...args) {
      super(...(args.length ? args : [clock]));
    }
    static now() {
      return clock;
    }
  }
  const schedule = (callback, delay = 0) => {
    const id = ++tid;
    timers.set(id, { callback, at: clock + Math.max(0, delay), delay });
    return id;
  };
  const advance = (milliseconds) => {
    const end = clock + milliseconds;
    let steps = 0;
    while (true) {
      const entry = [...timers]
        .filter(([, t]) => t.at <= end)
        .sort((a, b) => a[1].at - b[1].at || a[0] - b[0])[0];
      if (!entry) break;
      assert(++steps < 5000, "timer loop did not settle");
      const [id, timer] = entry;
      timers.delete(id);
      clock = timer.at;
      timer.callback();
    }
    clock = end;
  };
  let transform = { a: 1, d: 1, e: 0, f: 0 };
  const ctx = new Proxy(
    {
      measureText: (s) => ({ width: s.length * 7 }),
      clearRect: () => {
        rendered.length = 0;
      },
      setTransform: (a, b, c, d, e, f) => {
        transform = { a, d, e, f };
      },
      fillText: (s, x, y) => {
        texts.push(s);
        rendered.push({ label: s, x, y });
      },
    },
    { get: (o, k) => (k in o ? o[k] : () => {}) },
  );
  const wx = {
    createCanvas: () => ({ getContext: () => ctx }),
    getSystemInfoSync: () => ({
      windowWidth: 390,
      windowHeight: 844,
      pixelRatio: 1,
      ...options.info,
    }),
    getStorageSync: (k) => storage.get(k),
    getMenuButtonBoundingClientRect: () => options.capsule,
    setStorageSync: (k, v) => storage.set(k, v),
    removeStorageSync: (k) => storage.delete(k),
    setInnerAudioOption: (o) => audioOptions.push(o),
    createInnerAudioContext: () => {
      const context = {
        src: "",
        plays: [],
        stops: 0,
        destroys: 0,
        play() {
          this.plays.push({ src: this.src, at: clock });
        },
        stop() {
          this.stops++;
        },
        destroy() {
          this.destroys++;
        },
        onError(f) {
          this.error = f;
        },
      };
      audioContexts.push(context);
      return context;
    },
    vibrateShort: (o) => {
      vibrations.push({ options: o, at: clock });
      o?.success?.({});
    },
    connectSocket: (options) => {
      const listeners = {};
      const task = {
        options,
        sent: [],
        onOpen: (f) => (listeners.open = f),
        onMessage: (f) => (listeners.message = f),
        onClose: (f) => (listeners.close = f),
        onError: (f) => (listeners.error = f),
        send: (o) => task.sent.push(JSON.parse(o.data)),
        closeCount: 0,
        close: () => {
          task.closeCount++;
        },
        error: (reason) => listeners.error({ errMsg: reason }),
        opened: () => listeners.open({}),
        message: (m) => listeners.message({ data: JSON.stringify(m) }),
      };
      tasks.push(task);
      return task;
    },
    showShareMenu: () => {},
    showModal: (o) => {
      modals.push(o);
      if (options.confirm !== undefined)
        o.success?.({ confirm: options.confirm });
    },
    setClipboardData: (o) => {
      clipboard.push(o.data);
      o.success?.();
    },
    shareAppMessage: (o) => shares.push(o),
  };
  for (const event of [
    "TouchStart",
    "TouchMove",
    "TouchEnd",
    "TouchCancel",
    "Show",
    "Hide",
    "WindowResize",
    "ShareAppMessage",
  ])
    wx["on" + event] = (f) => (events[event] = f);
  vm.runInNewContext(
    readFileSync(new URL("../dist/game.js", import.meta.url), "utf8"),
    {
      wx,
      console,
      Date: FakeDate,
      setTimeout: schedule,
      clearTimeout: (id) => timers.delete(id),
    },
  );
  return {
    events,
    texts,
    storage,
    tasks,
    rendered,
    modals,
    clipboard,
    shares,
    timers,
    advance,
    audioContexts,
    audioOptions,
    vibrations,
    button: (label) => {
      const t = rendered.find((t) => t.label === label);
      assert(t, `Missing button ${label}`);
      events.TouchStart({
        touches: [
          {
            clientX: (t.x + 4) * transform.a + transform.e,
            clientY: t.y * transform.d + transform.f,
          },
        ],
      });
      events.TouchEnd({});
    },
    touch: (x, y) => {
      events.TouchStart({
        touches: [
          {
            clientX: x * transform.a + transform.e,
            clientY: y * transform.d + transform.f,
          },
        ],
      });
      events.TouchEnd({});
    },
    drag: (x, y, dx, dy) => {
      events.TouchStart({
        touches: [
          {
            clientX: x * transform.a + transform.e,
            clientY: y * transform.d + transform.f,
          },
        ],
      });
      events.TouchMove({
        touches: [
          {
            clientX: dx * transform.a + transform.e,
            clientY: dy * transform.d + transform.f,
          },
        ],
      });
      events.TouchEnd({});
    },
  };
}
function snapshot() {
  return {
    roomId: "test",
    phase: "play_trick",
    version: 1,
    seats: Array.from({ length: 4 }, (_, i) => ({
      name: "玩家" + i,
      uid: "" + i,
      online: true,
      handCount: 25,
    })),
    teams: [{ levelRank: "2" }, { levelRank: "2" }],
    mySeat: 0,
    myHand: [[{ id: 1, suit: "♠️", rank: "A", suit_class: "黑桃" }]],
    roundIndex: 0,
    points: 0,
    starterSeat: 0,
    trump: { levelRank: "2", hasTrumpSuit: false, locked: false },
    trick: {
      leaderSeat: 0,
      turnSeat: 0,
      playedMoves: [null, null, null, null],
      lastMoves: [null, null, null, null],
      resolved: false,
    },
    callPassedSeats: [false, false, false, false],
    surrenderVotes: [false, false, false, false],
    usedTrumpIds: [],
  };
}
test("bundle starts without DOM and connects from lobby", () => {
  const s = boot();
  assert(s.texts.includes("升级 · 好友房"));
  s.button("加入房间");
  assert.equal(s.tasks.length, 1);
  assert(s.tasks[0].options.url.startsWith("ws://"));
  s.tasks[0].message({
    type: "hello",
    token: "signed",
    name: "牌友",
    uid: "one",
  });
  assert([...s.storage.values()].includes("signed"));
});

test("notch, gesture bar and capsule insets preserve touch coordinates", () => {
  const s = boot({
    info: {
      windowWidth: 844,
      windowHeight: 390,
      pixelRatio: 1,
      safeArea: { left: 44, right: 800, top: 0, bottom: 369 },
    },
    capsule: { left: 740, right: 830, top: 6, bottom: 38 },
  });
  s.button("规则");
  assert(s.rendered.some((t) => t.label === "从入座到开局"));
  s.button("关闭");
  s.button("加入房间");
  assert.equal(s.tasks.length, 1);
});

test("rules modal blocks table touches; invitation copies only room code", () => {
  const s = boot();
  s.button("规则");
  assert(s.rendered.some((t) => t.label === "从入座到开局"));
  s.button("加入房间");
  assert.equal(s.tasks.length, 0);
  s.button("下一页");
  assert(s.rendered.some((t) => t.label === "定主、扣底与攻改"));
  s.button("关闭");
  s.button("加入房间");
  const task = s.tasks[0];
  task.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  task.message({ type: "snapshot", state: snapshot() });
  s.button("菜单");
  s.button("复制房间码");
  assert.deepEqual(s.clipboard, ["test"]);
  s.button("邀请好友");
  assert.equal(s.shares[0].query, "room=test");
  assert(!JSON.stringify(s.shares).includes("signed"));
});

test("leaving an active round requires confirmation and cancellation keeps connection", () => {
  const s = boot({ confirm: false });
  s.button("加入房间");
  const t = s.tasks[0];
  t.opened();
  t.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  t.message({ type: "snapshot", state: snapshot() });
  s.button("返回大厅");
  assert.equal(s.modals.length, 1);
  assert.equal(t.closeCount, 0);
  assert(s.rendered.some((t) => t.label === "出牌"));
});
test("touch sends server command; foreground restores token", () => {
  const s = boot();
  s.button("加入房间");
  const task = s.tasks[0];
  task.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  task.message({ type: "snapshot", state: snapshot() });
  s.touch(38, 480);
  s.events.TouchEnd({});
  s.button("出牌");
  assert.equal(task.sent[0].type, "game.play_cards");
  assert.equal(JSON.stringify(task.sent[0].payload.cardIds), "[1]");
  assert(task.sent[0].reqId);
  task.message({ type: "ack", reqId: task.sent[0].reqId, ok: true });
  s.events.Show({});
  assert.equal(s.tasks.length, 2);
  assert(s.tasks[1].options.url.includes("token=signed"));
});

test("failed unopened tasks are not closed and stale errors do not retry twice", () => {
  const s = boot();
  s.button("加入房间");
  const task = s.tasks[0];
  task.error("connection refused");
  task.error("connection refused");
  assert.equal(task.closeCount, 0);
  assert(s.texts.some((t) => t.includes("正在重试")));
  s.events.Show({});
  assert.equal(task.closeCount, 0);
});

test("pair shortcut submits both physical cards; history mode cannot submit moves", () => {
  const s = boot();
  s.button("加入房间");
  const t = s.tasks[0];
  t.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  const state = snapshot();
  state.trickIndex = 1;
  state.myHand[0].push({ ...state.myHand[0][0], id: 55 });
  state.trick.lastMoves[0] = {
    seat: 0,
    suitClass: "黑桃",
    followMove: { cards: [state.myHand[0][0]] },
  };
  state.trick.winnerSeat = 0;
  t.message({ type: "snapshot", state });
  s.button("看上一墩");
  s.button("出牌");
  assert.equal(t.sent.length, 0);
  assert(s.rendered.some((t) => t.label.includes("上一墩回放")));
  s.button("回当前墩");
  s.button("选对子");
  s.button("出牌");
  assert.equal(JSON.stringify(t.sent[0].payload.cardIds), "[1,55]");
  t.message({ type: "ack", reqId: t.sent[0].reqId, ok: true });
  s.button("看上一墩");
  state.trickIndex++;
  state.version++;
  t.message({ type: "snapshot", state });
  assert(s.rendered.some((t) => t.label === "看上一墩"));
});

test("late open from a replaced task is closed; manual recovery retains request id", () => {
  const s = boot();
  s.button("加入房间");
  const old = s.tasks[0];
  old.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  old.message({ type: "snapshot", state: snapshot() });
  s.touch(38, 480);
  s.button("出牌");
  const id = old.sent[0].reqId;
  old.error("connection refused");
  s.button("连接中断 · 点击重连");
  old.opened();
  assert.equal(old.closeCount, 1);
  const next = s.tasks[1];
  next.opened();
  next.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  assert.equal(next.sent[0].reqId, id);
  // An acknowledgement from the abandoned socket cannot unlock the new request.
  old.message({ type: "ack", reqId: id, ok: true });
  s.button("出牌");
  assert.equal(next.sent.length, 1);
  next.message({ type: "ack", reqId: id, ok: true });
});

test("long moves expand without accidental submission and error messages remain readable", () => {
  const s = boot();
  s.button("加入房间");
  const t = s.tasks[0];
  t.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  const state = snapshot();
  state.trick.playedMoves[2] = {
    seat: 2,
    suitClass: "黑桃",
    info: "甩牌被阻止，强制出最小结构",
    followMove: {
      cards: Array.from({ length: 20 }, (_, i) => ({
        id: i,
        suit: "♠️",
        rank: "A",
        suit_class: "黑桃",
      })),
    },
  };
  t.message({ type: "snapshot", state });
  s.touch(40, 260);
  assert(s.rendered.some((t) => t.label === "出牌详情"));
  assert(s.rendered.some((t) => t.label === "对家 · 20 张"));
  assert.equal(t.sent.length, 0);
  s.button("关闭");
  t.message({ type: "error", message: "这不是你的回合" });
  s.button("菜单");
  s.button("消息记录");
  assert(s.rendered.some((t) => t.label === "牌桌消息"));
  assert(s.rendered.some((t) => t.label === "这不是你的回合"));
});

test("spectators see public settlement but cannot start next round", () => {
  const s = boot();
  s.button("加入房间");
  const t = s.tasks[0];
  t.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  const state = {
    ...snapshot(),
    phase: "round_settle",
    mySeat: -1,
    myHand: null,
    roundPointsFinal: 125,
    roundResultLabel: "过大关",
    callerDelta: 0,
    defenderDelta: 1,
    nextStarterSeat: 0,
    bottomPoints: 10,
    bottomMul: 2,
    bottomAward: 20,
    bottomReveal: [{ id: 5, suit: "♥️", rank: "5" }],
  };
  t.message({ type: "snapshot", state });
  assert(s.rendered.some((t) => t.label === "125"));
  assert(s.rendered.some((t) => t.label.includes("抠底实得 20 分")));
  assert(!s.rendered.some((t) => t.label === "下一局"));
  assert(s.rendered.some((t) => t.label.includes("开启下一局")));
  assert(!s.rendered.some((t) => t.label.includes("你的座位已保留")));
  assert.equal(t.sent.length, 0);
});

test("record panel respects the server hideRecord flag", () => {
  const s = boot();
  s.button("加入房间");
  const t = s.tasks[0];
  t.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  const state = {
    ...snapshot(),
    hideRecord: true,
    record: { bigJoker: 2, smallJoker: 1 },
  };
  t.message({ type: "snapshot", state });
  s.button("菜单");
  assert(!s.rendered.some((t) => t.label === "公开记牌"));
  assert(!s.rendered.some((t) => t.label.includes("大王已出")));
});

test("history exits when the next lead clears lastMoves without changing trick index", () => {
  const s = boot();
  s.button("加入房间");
  const t = s.tasks[0];
  t.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  const state = snapshot();
  state.trickIndex = 3;
  state.trick.lastMoves[0] = {
    seat: 0,
    suitClass: "黑桃",
    followMove: { cards: [state.myHand[0][0]] },
  };
  t.message({ type: "snapshot", state });
  s.button("看上一墩");
  state.version++;
  state.trick.lastMoves = [null, null, null, null];
  t.message({ type: "snapshot", state });
  assert(!s.rendered.some((t) => t.label.includes("上一墩回放")));
  s.touch(38, 480);
  s.button("出牌");
  assert.equal(t.sent.length, 1);
});

const preferenceKey = "shengji-preferences-v1";
function connected(options = {}, state = snapshot()) {
  const s = boot(options);
  s.button("加入房间");
  const task = s.tasks[0];
  task.opened();
  task.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  task.message({ type: "snapshot", state });
  return { s, task };
}
const allPlays = (s) => s.audioContexts.flatMap((c) => c.plays);
const motionFrames = (s) =>
  [...s.timers.values()].filter((t) => t.delay === 33);
test("lobby and room settings are reachable; audio opt-in previews and persists", () => {
  const s = boot({
    storage: [[preferenceKey, { sound: false, vibration: false }]],
  });
  s.button("声音与动效");
  assert(s.rendered.some((t) => t.label === "声音与动效"));
  assert.equal(s.audioContexts.length, 0);
  s.touch(80, 236);
  s.events.TouchEnd({});
  assert.equal(s.storage.get(preferenceKey).sound, true);
  assert.equal(s.audioContexts.length, 1);
  assert.equal(
    allPlays(s).filter((p) => p.src.endsWith("/turn.wav")).length,
    1,
  );
  assert.equal(s.audioOptions[0].obeyMuteSwitch, true);
  s.touch(80, 452);
  s.events.TouchEnd({});
  assert.equal(s.storage.get(preferenceKey).reducedMotion, true);
  assert.equal(motionFrames(s).length, 0);
  s.advance(1000);
  s.button("试听音效");
  assert.equal(
    allPlays(s).filter((p) => p.src.endsWith("/turn.wav")).length,
    2,
  );
  s.button("关闭");
  s.button("加入房间");
  const task = s.tasks[0];
  task.opened();
  task.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  task.message({ type: "snapshot", state: snapshot() });
  s.button("菜单");
  s.button("声音与动效");
  assert(s.rendered.some((t) => t.label === "轻快音效"));
});
test("reduced motion schedules no animation; hide cancels motion and resume never catches up audio", () => {
  const state = snapshot();
  state.trick.turnSeat = 1;
  const { s, task } = connected(
    {
      storage: [
        [preferenceKey, { sound: true, vibration: true, reducedMotion: true }],
      ],
    },
    state,
  );
  s.advance(1000);
  s.touch(38, 480);
  s.events.TouchEnd({});
  assert.equal(motionFrames(s).length, 0);
  const moved = {
    ...state,
    version: 2,
    myHand: [
      [
        ...state.myHand[0],
        { id: 2, suit: "♣️", rank: "K", suit_class: "梅花" },
      ],
    ],
  };
  task.message({ type: "snapshot", state: moved });
  assert.equal(motionFrames(s).length, 0);
  const audible = allPlays(s).length;
  s.events.Hide({});
  assert.equal(motionFrames(s).length, 0);
  s.advance(1000);
  s.events.Show({});
  const resumed = s.tasks.at(-1);
  resumed.opened();
  resumed.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  resumed.message({
    type: "snapshot",
    state: { ...moved, version: 3, trick: { ...moved.trick, turnSeat: 0 } },
  });
  s.advance(500);
  assert.equal(allPlays(s).length, audible);
  assert.equal(s.vibrations.length, 0);
  assert.equal(motionFrames(s).length, 0);
  const active = connected();
  active.s.touch(38, 480);
  assert(motionFrames(active.s).length > 0);
  active.s.events.Hide({});
  assert.equal(motionFrames(active.s).length, 0);
});
test("authoritative new turn sounds and vibrates once; duplicate and reconnect snapshots stay quiet", () => {
  const state = snapshot();
  state.trick.turnSeat = 1;
  state.trickIndex = 0;
  const { s, task } = connected(
    {
      storage: [
        [preferenceKey, { sound: true, vibration: true, reducedMotion: false }],
      ],
    },
    state,
  );
  s.advance(2000);
  const before = allPlays(s).length;
  const mine = { ...state, version: 2, trick: { ...state.trick, turnSeat: 0 } };
  task.message({ type: "snapshot", state: mine });
  assert.equal(allPlays(s).length, before + 1);
  assert(allPlays(s).at(-1).src.endsWith("/turn.wav"));
  assert.equal(s.vibrations.length, 1);
  s.advance(2000);
  task.message({ type: "snapshot", state: mine });
  assert.equal(allPlays(s).length, before + 1);
  assert.equal(s.vibrations.length, 1);
  s.events.Hide({});
  s.events.Show({});
  const resumed = s.tasks.at(-1);
  resumed.opened();
  resumed.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  resumed.message({ type: "snapshot", state: { ...mine, version: 3 } });
  s.advance(1000);
  assert.equal(allPlays(s).length, before + 1);
  assert.equal(s.vibrations.length, 1);
});
test("snapshots behind a modal never play feedback or enable underlying buttons", () => {
  const state = snapshot();
  state.trick.turnSeat = 1;
  const { s, task } = connected(
    {
      storage: [
        [preferenceKey, { sound: true, vibration: true, reducedMotion: false }],
      ],
    },
    state,
  );
  s.advance(1000);
  s.button("规则");
  s.advance(1000);
  const before = allPlays(s).length;
  const mine = { ...state, version: 2, trick: { ...state.trick, turnSeat: 0 } };
  task.message({ type: "snapshot", state: mine });
  assert.equal(allPlays(s).length, before);
  assert.equal(s.vibrations.length, 0);
  assert.equal(motionFrames(s).length, 0);
  s.touch(40, 480);
  s.events.TouchEnd({});
  assert.equal(task.sent.length, 0);
  s.button("关闭");
  s.advance(1000);
  const closedCount = allPlays(s).length;
  task.message({ type: "snapshot", state: mine });
  assert.equal(allPlays(s).length, closedCount);
  assert.equal(s.vibrations.length, 0);
});
test("drag sounds are throttled and stable card hit boxes ignore intermediate animation", () => {
  const state = snapshot();
  state.myHand = [
    Array.from({ length: 3 }, (_, i) => ({
      id: i + 1,
      suit: "♠️",
      rank: ["A", "K", "Q"][i],
      suit_class: "黑桃",
    })),
  ];
  const { s, task } = connected(
    {
      storage: [
        [
          preferenceKey,
          { sound: true, vibration: false, reducedMotion: false },
        ],
      ],
    },
    state,
  );
  s.advance(1000);
  const start = allPlays(s).length;
  s.events.TouchStart({ touches: [{ clientX: 38, clientY: 480 }] });
  s.advance(33);
  s.events.TouchMove({ touches: [{ clientX: 82, clientY: 480 }] });
  s.advance(33);
  s.events.TouchMove({ touches: [{ clientX: 126, clientY: 480 }] });
  s.events.TouchMove({ touches: [{ clientX: 82, clientY: 480 }] });
  s.events.TouchEnd({});
  assert.equal(
    allPlays(s).length,
    start + 1,
    "three quick cards produce only one selection cue",
  );
  assert(motionFrames(s).length > 0);
  s.button("出牌");
  assert.equal(JSON.stringify(task.sent.at(-1).payload.cardIds), "[1,2,3]");
});

test("record option only configures newly created rooms", () => {
  for (const create of [false, true]) {
    const s = boot();
    s.button("新房记牌器：关闭");
    s.button(create ? "创建房间" : "加入房间");
    const url = new URL(s.tasks[0].options.url);
    assert.equal(url.searchParams.get("record"), create ? "1" : "0");
  }
});

test("public score toast reaches players and spectators even while room menu is open", () => {
  for (const mySeat of [0, 1, 2, 3, -1]) {
    const state = {
      ...snapshot(),
      mySeat,
      myHand: mySeat < 0 ? null : snapshot().myHand,
    };
    const { s, task } = connected({}, state);
    s.button("菜单");
    task.message({
      type: "snapshot",
      state: { ...state, version: 2, points: 20 },
    });
    assert(s.rendered.some((t) => t.label === "打家 +20 分"));
    s.advance(2300);
    assert(!s.rendered.some((t) => t.label === "打家 +20 分"));
  }
});

test("play errors float briefly instead of occupying the bottom strip", () => {
  const { s, task } = connected();
  task.message({ type: "error", message: "请跟足黑桃" });
  const notices = s.rendered.filter((t) => t.label === "请跟足黑桃");
  assert.equal(notices.length, 1);
  assert.equal(notices[0].x, 30);
  s.advance(3300);
  assert(!s.rendered.some((t) => t.label === "请跟足黑桃"));
});

test("lobby practice controls send add and remove, with capacity guards", () => {
  const state = {
    ...snapshot(),
    phase: "lobby",
    seats: snapshot().seats.map((p, i) =>
      i < 2
        ? { ...p, isBot: i === 1, ready: false }
        : { ...p, uid: "", ready: false },
    ),
  };
  const { s, task } = connected({}, state);
  s.button("增加机器人");
  assert.equal(task.sent.at(-1).type, "room.add_bot");
  task.message({ type: "ack", reqId: task.sent.at(-1).reqId, ok: true });
  task.message({ type: "snapshot", state: { ...state, version: 2 } });
  s.button("减少机器人");
  assert.equal(task.sent.at(-1).type, "room.remove_bot");
  task.message({ type: "ack", reqId: task.sent.at(-1).reqId, ok: true });
  const count = task.sent.length;
  task.message({
    type: "snapshot",
    state: { ...state, version: 3, seats: snapshot().seats },
  });
  s.button("增加机器人");
  s.button("减少机器人");
  assert.equal(
    task.sent.length,
    count,
    "full human room must disable both controls",
  );
});

test("portrait hand scroll reveals the 33rd card without selecting during swipe", () => {
  const state = snapshot();
  state.phase = "bottom";
  state.bottomOwnerSeat = 0;
  state.myHand = [
    Array.from({ length: 33 }, (_, i) => ({
      id: i + 1,
      suit: "♠️",
      rank: String((i % 9) + 2),
      suit_class: "主牌",
    })),
  ];
  const { s, task } = connected({}, state);
  s.drag(360, 680, 360, 480);
  assert(s.rendered.some((t) => t.label.startsWith("已选 0 / 33")));
  s.touch(38, 670);
  assert(s.rendered.some((t) => t.label.startsWith("已选 1 / 33")));
  assert.equal(task.sent.length, 0);
  s.button("清空选牌");
  assert(s.rendered.some((t) => t.label.startsWith("已选 0 / 33")));
});

test("portrait rules scroll stays modal and release after a swipe cannot press a button", () => {
  const s = boot({ info: { windowWidth: 320, windowHeight: 568 } });
  s.button("规则");
  s.drag(90, 300, 90, 180);
  assert.equal(s.tasks.length, 0);
  s.button("关闭");
  s.button("加入房间");
  assert.equal(s.tasks.length, 1);
});

test("shipped game requests portrait orientation", () => {
  assert.equal(
    JSON.parse(
      readFileSync(new URL("../dist/game.json", import.meta.url), "utf8"),
    ).deviceOrientation,
    "portrait",
  );
});
test("explicit server addresses remain visible instead of being silently replaced", () => {
  const s = boot({ storage: [["shengji-server", "ws://127.0.0.1:18088/ws"]] });
  s.button("加入房间");
  assert.equal(
    s.tasks.length,
    0,
    "loopback is explained as unavailable on a phone",
  );
  assert(s.texts.some((t) => t.includes("局域网 IP")));
  s.button("连接设置");
  assert(s.rendered.some((t) => t.label === "ws://127.0.0.1:18088/ws"));
});

test("lobby clockwise seats preserve backend indices before and after sitting", () => {
  const positions = [
    [40, 286],
    [230, 286],
    [230, 408],
    [40, 408],
  ];
  for (const mySeat of [-1, 2])
    for (let seat = 0; seat < 4; seat++) {
      if (seat === mySeat) continue;
      const state = {
        ...snapshot(),
        phase: "lobby",
        mySeat,
        seats: snapshot().seats.map((p, i) => ({
          ...p,
          uid: i === mySeat ? "me" : "",
          name: "",
          ready: false,
          team: i % 2,
        })),
      };
      const { s, task } = connected({}, state);
      s.touch(...positions[seat]);
      assert.equal(task.sent.at(-1).type, "room.sit");
      assert.equal(task.sent.at(-1).payload.seat, seat);
    }
});
