// Runs the shipped Canvas bundle in four isolated wx runtimes, using real WebSockets.
// Start a separate Go backend first; this intentionally is not part of npm test.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";

const endpoint = process.env.SHENGJI_TEST_URL;
if (!endpoint)
  throw Error("Set SHENGJI_TEST_URL to a LAN ws://IP:PORT/ws endpoint");
const room = `qa-${Date.now()}`;
const bundle = readFileSync(
  new URL("../dist/game.js", import.meta.url),
  "utf8",
);
const clients = [];
const wait = async (condition, label) => {
  const until = Date.now() + 10000;
  while (!condition()) {
    if (Date.now() > until) throw Error(`Timeout: ${label}`);
    await new Promise((r) => setTimeout(r, 10));
  }
};
function boot(index) {
  const events = {},
    texts = [],
    sockets = [],
    timers = new Set();
  const storage = new Map([
    ["shengji-server", endpoint],
    ["shengji-room", room],
    ["shengji-name", `QA${index}`],
  ]);
  const client = {
    events,
    texts,
    sockets,
    view: null,
    acks: [],
    sent: [],
    errors: [],
  };
  const ctx = new Proxy(
    {
      measureText: (s) => ({ width: s.length * 8 }),
      clearRect: () => {
        texts.length = 0;
      },
      fillText: (label, x, y) => texts.push({ label, x, y }),
    },
    { get: (o, k) => (k in o ? o[k] : () => {}) },
  );
  const wx = {
    createCanvas: () => ({ getContext: () => ctx }),
    getSystemInfoSync: () => ({
      windowWidth: 960,
      windowHeight: 540,
      pixelRatio: 1,
    }),
    getStorageSync: (k) => storage.get(k),
    setStorageSync: (k, v) => storage.set(k, v),
    removeStorageSync: (k) => storage.delete(k),
    showShareMenu() {},
    showModal: (o) => o.success?.({ confirm: true }),
    connectSocket: (options) => {
      const socket = new WebSocket(options.url);
      sockets.push(socket);
      return {
        onOpen: (f) => socket.addEventListener("open", f),
        onError: (f) =>
          socket.addEventListener("error", () =>
            f({ errMsg: "network error" }),
          ),
        onClose: (f) => socket.addEventListener("close", f),
        onMessage: (f) =>
          socket.addEventListener("message", (e) => {
            const msg = JSON.parse(e.data);
            if (msg.type === "snapshot") client.view = msg.state;
            if (msg.type === "ack") client.acks.push(msg);
            if (msg.type === "error") client.errors.push(msg);
            f({ data: e.data });
          }),
        send: (o) => {
          client.sent.push(JSON.parse(o.data));
          socket.send(o.data);
        },
        close: () => socket.close(),
      };
    },
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
    setTimeout: (f, ms) => {
      const t = setTimeout(f, ms);
      timers.add(t);
      return t;
    },
    clearTimeout,
  });
  client.touch = (x, y) => {
    events.TouchStart({ touches: [{ clientX: x, clientY: y }] });
    events.TouchEnd({});
  };
  client.button = (label) => {
    const t = texts.find((t) => t.label === label);
    assert(t, `Missing button ${label}; phase=${client.view?.phase}`);
    client.touch(t.x + 8, t.y);
  };
  client.cleanup = () => {
    if (texts.some((t) => t.label === "返回大厅")) client.button("返回大厅");
    timers.forEach(clearTimeout);
    sockets.forEach((s) => s.close());
  };
  clients.push(client);
  return client;
}
async function action(c, label) {
  const count = c.acks.length;
  c.button(label);
  await wait(() => c.acks.length > count, label);
  const ack = c.acks.at(-1);
  assert.equal(ack.ok, true, `${label}: ${JSON.stringify(ack)}`);
  const version = c.view.version;
  await wait(
    () => clients.every((p) => p.view?.version >= version),
    "broadcast",
  );
}
function select(c, ids) {
  const hand = c.view.myHand.flat();
  const step = Math.min(42, 850 / Math.max(1, hand.length));
  for (const id of ids) {
    const i = hand.findIndex((card) => card.id === id);
    assert(i >= 0);
    c.touch(33 + i * step, 405);
  }
}
try {
  for (let i = 0; i < 4; i++) {
    const c = boot(i);
    c.button("加入房间");
    await wait(() => c.view, "initial snapshot");
    // Spectator layout uses a fixed compass; choose the matching visible seat.
    const positions = [
      [344, 318],
      [672, 254],
      [344, 216],
      [38, 254],
    ];
    const count = c.acks.length;
    c.touch(...positions[i]);
    await wait(() => c.acks.length > count && c.view.mySeat === i, "sit");
    assert.equal(c.acks.at(-1).ok, true);
  }
  for (const c of clients) await action(c, "准备");
  await wait(() => clients.every((c) => c.view.phase === "call_trump"), "deal");
  assert.equal(
    new Set(clients.flatMap((c) => c.view.myHand.flat().map((c) => c.id))).size,
    100,
  );
  let called = false;
  if (process.env.SHENGJI_TEST_CALL === "1") {
    for (const c of clients) {
      const hand = c.view.myHand.flat();
      const joker = hand.find(
        (j) =>
          ["BJ", "SJ"].includes(j.suit) &&
          hand.some(
            (l) =>
              l.rank === "2" &&
              (j.suit === "BJ" ? /[♥♦]/ : /[♠♣]/).test(l.suit),
          ),
      );
      if (!joker) continue;
      const level = hand.find(
        (l) =>
          l.rank === "2" &&
          (joker.suit === "BJ" ? /[♥♦]/ : /[♠♣]/).test(l.suit),
      );
      select(c, [joker.id, level.id]);
      await action(c, "定主");
      assert.equal(c.view.phase, "bottom");
      assert.equal(c.view.myHand.flat().length, 33);
      select(
        c,
        c.view.myHand
          .flat()
          .slice(0, 8)
          .map((c) => c.id),
      );
      await action(c, "扣底 8 张");
      while (clients[0].view.phase === "trump_fight")
        await action(clients[clients[0].view.fightTurnSeat], "跳过");
      assert.equal(clients[0].view.trick.turnSeat, c.view.mySeat);
      called = true;
      break;
    }
    assert(called, "Random deal has no callable hand; rerun this scenario");
  } else for (const c of clients) await action(c, "跳过");
  await wait(
    () => clients.every((c) => c.view.phase === "play_trick"),
    "start play",
  );
  const originalSeat = clients[2].view.mySeat;
  clients[2].events.Hide({});
  clients[2].events.Show({});
  await wait(
    () =>
      clients[2].sockets.length === 2 &&
      clients[2].sockets[1].readyState === 1 &&
      clients[2].view.mySeat === originalSeat,
    "resume",
  );
  let plays = 0;
  while (clients[0].view.phase === "play_trick") {
    const v = clients[0].view;
    const c = clients[v.trick.turnSeat];
    await wait(() => c.view.version >= v.version, "turn snapshot");
    const own = c.view;
    const lead = own.trick.playedMoves[own.trick.leaderSeat];
    const hand = own.myHand.flat();
    const card =
      hand.find((card) => !lead || card.suit_class === lead.suitClass) ??
      hand[0];
    select(c, [card.id]);
    await action(c, "出牌");
    if (++plays > 100) throw Error("Round failed to end");
  }
  const end = clients[0].view;
  assert.equal(plays, 100);
  assert.equal(end.phase, "round_settle");
  assert.equal(end.bottomRevealed, true);
  assert.equal(end.bottomReveal.length, 8);
  assert(clients.every((c) => (c.view.myHand ?? []).flat().length === 0));
  assert(
    clients.every((c) => c.view.roundPointsFinal === end.roundPointsFinal),
  );
  console.log(
    JSON.stringify({
      room,
      called,
      plays,
      tricks: end.trickIndex,
      points: end.roundPointsFinal,
      result: end.roundResultLabel,
      bottomPoints: end.bottomPoints,
      bottomAward: end.bottomAward,
      resumedSeat: originalSeat,
    }),
  );
  await action(clients[end.nextStarterSeat], "下一局");
  await wait(
    () => clients.every((c) => c.view.phase === "call_trump"),
    "next round",
  );
  assert.equal(clients[0].view.roundIndex, end.roundIndex + 1);
  assert.equal(clients[0].view.bottomAward, 0);
  while (clients[0].view.phase === "call_trump") {
    const seat = clients[0].view.callTurnSeat;
    await action(clients[seat], "跳过");
  }
  await wait(
    () => clients.every((c) => c.view.phase === "play_trick"),
    "second round play",
  );
  const before = clients[clients[0].view.trick.turnSeat];
  const sent = before.sent.length;
  before.button("出牌");
  assert.equal(
    before.sent.length,
    sent,
    "empty selection must be blocked locally",
  );
  assert(before.texts.some((t) => t.label === "请先选择手牌"));
  await action(clients[0], "申请弃局");
  assert.equal(
    clients[0].view.phase,
    "play_trick",
    "one vote must not end round",
  );
  await action(clients[2], "申请弃局");
  await wait(
    () => clients.every((c) => c.view.phase === "round_settle"),
    "surrender settlement",
  );
  assert.equal(clients[0].view.surrendered, true);
  assert.equal(clients[0].view.bottomAward, 0);
  console.log(
    "PASS: four bundled clients, touch commands, full round, resume, settlement, next round, empty selection, teammate surrender",
  );
} finally {
  clients.forEach((c) => c.cleanup());
}
