import { test } from "node:test";
import assert from "node:assert/strict";
import { buildSync } from "esbuild";
import vm from "node:vm";
import { fileURLToPath } from "node:url";
import { scene, deck } from "./scenes.mjs";
const module = { exports: {} };
vm.runInNewContext(
  buildSync({
    entryPoints: [
      fileURLToPath(new URL("../src/feedback.ts", import.meta.url)),
    ],
    bundle: true,
    platform: "node",
    format: "cjs",
    write: false,
  }).outputFiles[0].text,
  { module, exports: module.exports },
);
const { FeedbackTracker, lastTrickSummary } = module.exports;
const kinds = (events) => Array.from(events, (e) => e.kind);
const move = (seat, id) => ({ seat, followMove: { cards: [deck[id]] } });
test("first snapshot, duplicates and reconnect baseline never replay events", () => {
  const t = new FeedbackTracker(),
    s = scene();
  assert.equal(t.observe(s).length, 0);
  assert.equal(t.observe(structuredClone(s)).length, 0);
  assert.equal(t.observe({ ...s, version: 10 }).length, 0);
  t.reset();
  assert.equal(t.observe(scene("round_settle", { callerDelta: 2 })).length, 0);
  assert.equal(t.observe(scene("play_trick", { version: 0 })).length, 0);
});
test("deal only animates newly received own cards; spectators and seat changes are quiet", () => {
  const t = new FeedbackTracker();
  t.observe(scene("lobby"));
  const next = scene("call_trump", { version: 2 });
  assert.equal(t.observe(next).find((e) => e.kind === "hand").ids.length, 25);
  const bottom = scene("bottom", { version: 3 });
  assert.equal(t.observe(bottom).find((e) => e.kind === "hand").ids.length, 8);
  assert.equal(
    t.observe(scene("bottom", { version: 4, mySeat: -1, myHand: null })).length,
    0,
  );
});
test("only new confirmed physical moves animate, including fourth move in lastMoves", () => {
  const t = new FeedbackTracker(),
    s = scene();
  t.observe(s);
  const played = [move(0, 0), null, null, null];
  const first = {
    ...s,
    version: 2,
    trick: { ...s.trick, turnSeat: 1, playedMoves: played },
  };
  assert.equal(t.observe(first).filter((e) => e.kind === "move").length, 1);
  assert.equal(t.observe({ ...first, version: 3 }).length, 0);
  const three = {
    ...first,
    version: 4,
    trick: {
      ...first.trick,
      playedMoves: [move(0, 0), move(1, 1), move(2, 2), null],
    },
  };
  t.observe(three);
  const resolved = {
    ...s,
    version: 5,
    trickIndex: s.trickIndex + 1,
    trick: {
      ...s.trick,
      resolved: true,
      lastMoves: [move(0, 0), move(1, 1), move(2, 2), move(3, 3)],
    },
  };
  const effects = t.observe(resolved);
  assert.deepEqual(
    Array.from(
      effects.filter((e) => e.kind === "move"),
      (e) => e.seat,
    ),
    [3],
  );
  assert(kinds(effects).includes("trick"));
  assert.equal(t.observe({ ...resolved, version: 6 }).length, 0);
});
test("turn cues require a new actionable turn; passed and race callers are not prompted", () => {
  const t = new FeedbackTracker(),
    s = scene();
  t.observe({ ...s, trick: { ...s.trick, turnSeat: 1 } });
  assert(kinds(t.observe({ ...s, version: 2 })).includes("turn"));
  assert.equal(t.observe({ ...s, version: 3 }).length, 0);
  const race = scene("call_trump", { version: 4, callMode: "race" });
  assert(!kinds(t.observe(race)).includes("turn"));
  const passed = scene("call_trump", {
    version: 5,
    callPassedSeats: [true, false, false, false],
  });
  assert(!kinds(t.observe(passed)).includes("turn"));
});
test("celebration respects actual teams, spectators, surrender and duplicate settlements", () => {
  for (const [overrides, expected] of [
    [{ callerDelta: 2 }, true],
    [{ callerDelta: 0 }, false],
    [{ callerDelta: 2, surrendered: true }, false],
    [{ mySeat: -1 }, false],
  ]) {
    const t = new FeedbackTracker();
    t.observe(scene("play_trick", { mySeat: overrides.mySeat ?? 0 }));
    const final = scene("round_settle", { version: 2, ...overrides });
    assert.equal(kinds(t.observe(final)).includes("celebrate"), expected);
    assert.equal(t.observe({ ...final, version: 3 }).length, 0);
  }
  const t = new FeedbackTracker();
  t.observe(scene());
  assert(
    !kinds(
      t.observe(
        scene("game_over", { version: 2, winnerTeam: 1, callerDelta: 3 }),
      ),
    ).includes("celebrate"),
  );
});
test("trick summary describes public card points without implying defender gains on caller wins", () => {
  const s = scene();
  s.trick.lastMoves = Array.from({ length: 4 }, (_, seat) => ({
    seat,
    followMove: { cards: [{ id: seat, rank: seat ? "A" : "K" }] },
  }));
  s.trick.winnerSeat = 0;
  assert.equal(lastTrickSummary(s), "坐家守住 10 分");
  s.trick.winnerSeat = 1;
  assert.equal(lastTrickSummary(s), "打家收得 10 分");
  s.bottomAward = 80;
  s.roundPointsFinal = 130;
  assert.equal(lastTrickSummary(s), "打家收得 10 分");
  s.trick.lastMoves[0] = null;
  assert.equal(lastTrickSummary(s), "");
});
