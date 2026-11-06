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
      windowWidth: 960,
      windowHeight: 540,
      pixelRatio: 1,
      ...options.info,
    }),
    getStorageSync: (k) => storage.get(k),
    getMenuButtonBoundingClientRect: () => options.capsule,
    setStorageSync: (k, v) => storage.set(k, v),
    removeStorageSync: (k) => storage.delete(k),
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
    { wx, console, setTimeout: () => ++tid, clearTimeout: () => {} },
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
    touch: (x, y) =>
      events.TouchStart({ touches: [{ clientX: x, clientY: y }] }),
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
  assert(s.texts.includes("升 级 · 好友房"));
  s.touch(720, 365);
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
  s.touch(720, 365);
  assert.equal(s.tasks.length, 0);
  s.button("下一页");
  assert(s.rendered.some((t) => t.label === "定主、扣底与攻改"));
  s.button("关闭");
  s.button("加入房间");
  const task = s.tasks[0];
  task.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  task.message({ type: "snapshot", state: snapshot() });
  s.button("房间");
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
  s.touch(720, 365);
  const task = s.tasks[0];
  task.message({ type: "hello", token: "signed", name: "牌友", uid: "one" });
  task.message({ type: "snapshot", state: snapshot() });
  s.touch(40, 400);
  s.events.TouchEnd({});
  s.touch(200, 510);
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
  s.touch(720, 365);
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
  s.touch(40, 400);
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
