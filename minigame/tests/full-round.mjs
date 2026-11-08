// Runs the shipped Canvas bundle in four isolated wx runtimes, using real WebSockets.
// Start a separate Go backend first; this intentionally is not part of npm test.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import { buildSync } from "esbuild";
import { fileURLToPath } from "node:url";
const layoutModule = { exports: {} };
vm.runInNewContext(
  buildSync({
    entryPoints: [
      fileURLToPath(new URL("../src/hand-layout.ts", import.meta.url)),
    ],
    bundle: true,
    format: "cjs",
    write: false,
  }).outputFiles[0].text,
  { module: layoutModule, exports: layoutModule.exports },
);
const { handLayout } = layoutModule.exports;

const endpoint = process.env.SHENGJI_TEST_URL;
if (!endpoint)
  throw Error("Set SHENGJI_TEST_URL to a LAN ws://IP:PORT/ws endpoint");
const room = `qa-${Date.now()}`;
const bundle = readFileSync(
  new URL("../dist/game.js", import.meta.url),
  "utf8",
);
const clients = [];
const usePairs = process.env.SHENGJI_TEST_PAIRS === "1";
const pairIn = (cards) =>
  cards.find((c) =>
    cards.some((d) => c.id !== d.id && c.suit === d.suit && c.rank === d.rank),
  );
const pointsOf = (cards) =>
  cards.reduce(
    (sum, c) =>
      sum + (c.rank === "5" ? 5 : ["10", "K"].includes(c.rank) ? 10 : 0),
    0,
  );
let observer;
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
    snapshots: new Map(),
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
      windowWidth: 390,
      windowHeight: 844,
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
            if (msg.type === "snapshot") {
              client.snapshots.set(socket, msg.state);
              // Late snapshots from a replaced connection cannot satisfy recovery.
              if (socket === sockets.at(-1)) client.view = msg.state;
            }
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
  client.drag = (x, y, dx, dy) => {
    events.TouchStart({ touches: [{ clientX: x, clientY: y }] });
    events.TouchMove({ touches: [{ clientX: dx, clientY: dy }] });
    events.TouchEnd({});
  };
  client.button = (label) => {
    if (label === "申请弃局") {
      client.button("菜单");
    }
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
  const layout = handLayout(
    hand.length,
    444,
    c.view.myHand.map((g) => g.length),
  );
  const maxScroll = Math.max(0, layout.contentHeight - 282);
  // Scroll to the top without selecting cards, then bring each target into view.
  for (let i = 0; i < Math.ceil(maxScroll / 180) + 1; i++)
    c.drag(360, 490, 360, 670);
  let scroll = 0;
  for (const id of ids) {
    const index = hand.findIndex((card) => card.id === id);
    assert(index >= 0);
    const slot = layout.slot(index);
    const desired = Math.min(maxScroll, Math.max(0, slot.y - 480));
    while (Math.abs(desired - scroll) > 0.01) {
      const delta = Math.max(-180, Math.min(180, desired - scroll));
      c.drag(
        360,
        delta > 0 ? 670 : 490,
        360,
        delta > 0 ? 670 - delta : 490 - delta,
      );
      scroll += delta;
    }
    c.touch(slot.x + 14, slot.y - scroll + 30);
  }
}
try {
  for (let i = 0; i < 4; i++) {
    const c = boot(i);
    c.button("加入房间");
    await wait(() => c.view, "initial snapshot");
    // Spectator layout uses a fixed compass; choose the matching visible seat.
    const positions = [
      [230, 408],
      [40, 408],
      [40, 286],
      [230, 286],
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
      const buryingHand = c.view.myHand.flat();
      const secondRowStart = Math.ceil(buryingHand.length / 2);
      // Exercise both rows, rather than accidentally testing only the first row.
      const buriedIds = [
        ...buryingHand.slice(0, 4),
        ...buryingHand.slice(secondRowStart, secondRowStart + 4),
      ].map((card) => card.id);
      select(c, buriedIds);
      await action(c, "扣底 8 张");
      assert.deepEqual(
        [...c.sent.at(-1).payload.discardIds].sort((a, b) => a - b),
        [...buriedIds].sort((a, b) => a - b),
      );
      assert.equal(c.view.myHand.flat().length, 25);
      assert(
        !c.view.myHand.flat().some((card) => buriedIds.includes(card.id)),
        "second-row cards must actually leave the hand",
      );
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
  observer = new WebSocket(
    `${endpoint}?room=${encodeURIComponent(room)}&name=observer`,
  );
  let observerView;
  observer.addEventListener("message", (e) => {
    const m = JSON.parse(e.data);
    if (m.type === "snapshot") observerView = m.state;
  });
  await wait(() => observerView, "observer snapshot");
  assert.equal(observerView.mySeat, -1);
  assert.equal(observerView.myHand, null);
  assert.equal(observerView.myBottom, null);
  assert(
    observerView.seats.every((s) => !Object.hasOwn(s, "hand")),
    "seat metadata must not contain private hands",
  );
  const wrong = clients[(clients[0].view.trick.turnSeat + 1) % 4];
  const beforeVersion = clients[0].view.version;
  wrong.sockets.at(-1).send(
    JSON.stringify({
      type: "game.play_cards",
      payload: { cardIds: [wrong.view.myHand.flat()[0].id] },
      reqId: "qa-wrong-turn",
    }),
  );
  await wait(
    () => wrong.acks.some((a) => a.reqId === "qa-wrong-turn"),
    "wrong-turn rejection",
  );
  assert.equal(wrong.acks.find((a) => a.reqId === "qa-wrong-turn").ok, false);
  assert.equal(clients[0].view.version, beforeVersion);
  const originalSeat = clients[2].view.mySeat;
  clients[2].events.Hide({});
  clients[2].events.Show({});
  await wait(
    () =>
      clients[2].sockets.length === 2 &&
      clients[2].sockets[1].readyState === 1 &&
      clients[2].snapshots.has(clients[2].sockets[1]) &&
      clients[2].view.mySeat === originalSeat,
    "resume",
  );
  let plays = 0,
    pairLeads = 0;
  const playedCards = new Map();
  while (clients[0].view.phase === "play_trick") {
    const v = clients[0].view;
    const c = clients[v.trick.turnSeat];
    await wait(() => c.view.version >= v.version, "turn snapshot");
    const own = c.view;
    const lead = own.trick.playedMoves[own.trick.leaderSeat];
    const hand = own.myHand.flat();
    let chosen;
    if (!lead) {
      const seed = usePairs ? pairIn(hand) : null;
      chosen = seed
        ? hand.filter((c) => c.suit === seed.suit && c.rank === seed.rank)
        : [hand[0]];
      if (chosen.length === 2) pairLeads++;
    } else {
      const count = lead.followMove.cards.length;
      const same = hand.filter((c) => c.suit_class === lead.suitClass);
      const seed = count === 2 && same.length >= 2 ? pairIn(same) : null;
      chosen = seed
        ? same.filter((c) => c.suit === seed.suit && c.rank === seed.rank)
        : same.slice(0, count);
      if (chosen.length < count)
        chosen.push(
          ...hand
            .filter((c) => !chosen.some((d) => c.id === d.id))
            .slice(0, count - chosen.length),
        );
    }
    for (const card of chosen) {
      assert(!playedCards.has(card.id), "physical card played twice");
      playedCards.set(card.id, card);
    }
    select(
      c,
      chosen.map((c) => c.id),
    );
    await action(c, "出牌");
    if (++plays > 100) throw Error("Round failed to end");
  }
  const end = clients[0].view;
  assert.equal(playedCards.size, 100);
  if (!usePairs) assert.equal(plays, 100);
  else assert(pairLeads > 0, "pairs mode must exercise a pair lead");
  assert.equal(end.phase, "round_settle");
  assert.equal(end.bottomRevealed, true);
  assert.equal(end.bottomReveal.length, 8);
  assert.equal(
    new Set([...playedCards.keys(), ...end.bottomReveal.map((c) => c.id)]).size,
    108,
  );
  assert.equal(pointsOf([...playedCards.values(), ...end.bottomReveal]), 200);
  const record = end.record;
  assert.equal(
    record.spade.num +
      record.heart.num +
      record.club.num +
      record.diamond.num +
      record.bigJoker +
      record.smallJoker,
    100,
  );
  for (const [key, suit] of [
    ["spade", "♠"],
    ["heart", "♥"],
    ["club", "♣"],
    ["diamond", "♦"],
  ]) {
    const cards = [...playedCards.values()].filter((c) =>
      c.suit.includes(suit),
    );
    assert.equal(record[key].num, cards.length);
    for (const [field, rank] of [
      ["a", "A"],
      ["k", "K"],
      ["ten", "10"],
    ])
      assert.equal(
        record[key][field],
        cards.filter((c) => c.rank === rank).length,
      );
  }
  assert(clients.every((c) => (c.view.myHand ?? []).flat().length === 0));
  assert(
    clients.every((c) => c.view.roundPointsFinal === end.roundPointsFinal),
  );
  console.log(
    JSON.stringify({
      room,
      called,
      plays,
      pairLeads,
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
  observer?.close();
  clients.forEach((c) => c.cleanup());
}
