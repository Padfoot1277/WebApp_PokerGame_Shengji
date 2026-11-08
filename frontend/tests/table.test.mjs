import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import ts from "typescript";
function load(source, extras = {}) {
  const module = { exports: {} };
  const context = vm.createContext({
    module,
    exports: module.exports,
    ...extras,
  });
  vm.runInContext(
    ts.transpileModule(source, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
      },
    }).outputText,
    context,
  );
  return context;
}
const table = load(
  readFileSync(new URL("../src/utils/table.ts", import.meta.url), "utf8"),
).module.exports;
test("settled cards remain until the next lead, with history available afterward", () => {
  const s = {
    phase: "play_trick",
    trick: { resolved: true, playedMoves: [null, null, null, null] },
  };
  assert.equal(table.showingLast(s), true);
  s.trick.resolved = false;
  s.trick.playedMoves[2] = { seat: 2 };
  assert.equal(table.showingLast(s), false);
  assert.equal(table.showingLast(s, true), true);
});
test("padding is one tag even for uniform off-suit cards", () => {
  for (const suitClass of ["杂牌", "Mix", "黑桃"]) {
    assert.equal(table.moveTag({ suitClass, info: "垫牌" }).label, "杂");
  }
  assert.equal(table.moveTag({ suitClass: "主牌", info: "" }).label, "主");
  for (const suitClass of ["方块", "红桃", "梅花", "黑桃"]) {
    assert.equal(table.moveTag({ suitClass, info: "" }).label, "副");
  }
});
const play = load(
  readFileSync(
    new URL("../src/utils/play-feedback.ts", import.meta.url),
    "utf8",
  ),
).module.exports;
test("hand selection waits for release and scrolling cancels the whole gesture", () => {
  const props = { selectedIds: [] };
  const script = readFileSync(
    new URL("../src/components/HandArea.vue", import.meta.url),
    "utf8",
  ).match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1];
  const game = {
    view: {
      phase: "play_trick",
      mySeat: 0,
      myHand: [[{ id: 7, suit: "♠", rank: "A", suit_class: "黑桃" }]],
      trick: {
        turnSeat: 0,
        leaderSeat: 0,
        playedMoves: [null, null, null, null],
      },
    },
    reviewPrevious: false,
    pending: false,
  };
  const ctx = load(script, {
    require: (name) =>
      name === "vue"
        ? {
            computed: (fn) => ({
              get value() {
                return fn();
              },
            }),
            watch() {},
          }
        : name.includes("store")
          ? { useGameStore: () => game }
          : name.includes("play-feedback")
            ? play
            : name.includes("utils/card")
              ? { suitToSymbol: (s) => ({ symbol: s }) }
              : {},
    defineProps: () => props,
    defineEmits: () => (name, ids) => {
      if (name === "update:selectedIds") props.selectedIds = ids;
    },
  });
  const event = (x, y, id = 7) => ({
    isPrimary: true,
    button: 0,
    clientX: x,
    clientY: y,
    target: { closest: () => ({ dataset: { cardId: String(id) } }) },
  });
  ctx.e = event(20, 20);
  vm.runInContext("down(e)", ctx);
  assert.equal(props.selectedIds.length, 0);
  ctx.e = event(20, 70);
  vm.runInContext("move(e); up(e)", ctx);
  assert.equal(props.selectedIds.length, 0);
  ctx.e = event(20, 20);
  vm.runInContext("down(e); up(e)", ctx);
  assert.equal(JSON.stringify(props.selectedIds), "[7]");
  props.selectedIds = [];
  ctx.e = event(20, 20, 7);
  vm.runInContext("down(e)", ctx);
  ctx.e = event(70, 20, 8);
  vm.runInContext("move(e); up(e)", ctx);
  assert.equal(
    JSON.stringify(props.selectedIds),
    "[7,8]",
    "horizontal painting selects both cards once",
  );
  props.selectedIds = [];
  ctx.e = event(20, 20, 7);
  vm.runInContext("down(e)", ctx);
  ctx.e = event(70, 20, 8);
  vm.runInContext("move(e)", ctx);
  ctx.e = event(70, 100, 8);
  vm.runInContext("move(e); up(e)", ctx);
  assert.equal(
    props.selectedIds.length,
    0,
    "vertical scrolling restores the pre-gesture selection",
  );
});

test("follow suit disables off-suit only when there are enough lead-suit cards", () => {
  const spade = { id: 1, suit_class: "黑桃" },
    heart = { id: 2, suit_class: "红桃" };
  const s = {
    phase: "play_trick",
    mySeat: 0,
    myHand: [[spade, heart]],
    trick: {
      turnSeat: 0,
      leaderSeat: 1,
      playedMoves: [
        null,
        { suitClass: "黑桃", followMove: { cards: [{}] } },
        null,
        null,
      ],
    },
  };
  assert.equal(play.unavailableCard(s, heart), true);
  assert.equal(play.unavailableCard(s, spade), false);
  s.trick.playedMoves[1].followMove.cards.push({});
  assert.equal(play.unavailableCard(s, heart), false);
});
test("floating points only follow a new authoritative snapshot", () => {
  const before = {
    roomId: "r",
    roundIndex: 0,
    version: 1,
    phase: "play_trick",
    points: 0,
    trump: {},
    trick: {
      playedMoves: [null, null, null, null],
      lastMoves: [null, null, null, null],
    },
  };
  const next = { ...before, version: 2, points: 20 };
  assert.equal(play.floatingFeedback(before, next)[0], "打家 +20 分");
  assert.equal(play.floatingFeedback(next, next).length, 0);
  assert.equal(play.floatingFeedback(null, next).length, 0);
});

test("public feedback is identical for all four seats and spectators", () => {
  const publicState = {
    roomId: "r",
    roundIndex: 0,
    version: 1,
    phase: "call_trump",
    points: 0,
    bottomOwnerSeat: -1,
    trump: { hasTrumpSuit: false },
    trick: {
      playedMoves: [null, null, null, null],
      lastMoves: [null, null, null, null],
    },
  };
  for (const mySeat of [0, 1, 2, 3, -1]) {
    const before = {
      ...publicState,
      mySeat,
      myHand: mySeat < 0 ? null : [[{ id: mySeat }]],
    };
    const trump = {
      ...before,
      version: 2,
      phase: "bottom",
      bottomOwnerSeat: 1,
      trump: { hasTrumpSuit: true, suit: "红桃", locked: true },
    };
    assert.equal(
      JSON.stringify(play.floatingFeedback(before, trump)),
      JSON.stringify(["定主 · 红桃 · 锁主"]),
    );
    const score = { ...trump, version: 3, points: 25 };
    assert.equal(
      JSON.stringify(play.floatingFeedback(trump, score)),
      JSON.stringify(["打家 +25 分"]),
    );
  }
});
