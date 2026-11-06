import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import { buildSync } from "esbuild";
const module = { exports: {} };
vm.runInNewContext(
  buildSync({
    entryPoints: [fileURLToPath(new URL("../src/audio.ts", import.meta.url))],
    bundle: true,
    format: "cjs",
    write: false,
  }).outputFiles[0].text,
  { module, exports: module.exports },
);
const { GameAudio } = module.exports;
function fixture() {
  let time = 0;
  const contexts = [];
  const audio = new GameAudio(
    {
      createInnerAudioContext() {
        const context = {
          plays: [],
          stops: 0,
          destroys: 0,
          src: "",
          play() {
            this.plays.push(this.src);
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
        contexts.push(context);
        return context;
      },
    },
    () => time,
  );
  return {
    audio,
    contexts,
    advance: (ms) => {
      time += ms;
    },
  };
}
test("default off, no allocation until enabled and a real touch unlock", () => {
  const { audio, contexts } = fixture();
  audio.unlock();
  assert.equal(contexts.length, 0);
  assert.equal(audio.play("tap"), false);
  audio.setEnabled(true);
  assert.equal(contexts.length, 0);
  assert.equal(audio.play("tap"), true);
  assert.equal(contexts.length, 1);
  assert.equal(contexts[0].obeyMuteSwitch, true);
  assert.equal(contexts[0].loop, false);
  assert(contexts[0].volume <= 0.3);
  assert.equal(contexts[0].autoplay, false);
  const locked = fixture();
  locked.audio.setEnabled(true);
  assert.equal(locked.audio.play("turn"), false);
  assert.equal(locked.contexts.length, 0);
});

test("modern runtimes use global mute option, only after user unlock", () => {
  const options = [];
  let created = 0;
  const audio = new GameAudio({
    setInnerAudioOption: (o) => options.push(o),
    createInnerAudioContext: () => {
      created++;
      return {
        src: "",
        autoplay: false,
        loop: false,
        obeyMuteSwitch: true,
        volume: 0,
        startTime: 0,
        play() {},
        stop() {},
        destroy() {},
      };
    },
  });
  audio.setEnabled(true);
  assert.equal(options.length, 0);
  audio.unlock();
  assert.equal(options.length, 1);
  assert.equal(options[0].obeyMuteSwitch, true);
  assert.equal(options[0].mixWithOther, true);
  assert.equal(created, 1);
  options[0].fail();
  assert.equal(audio.play("tap"), false);
});
test("rapid selection is throttled and low-priority taps never cut an important cue", () => {
  const { audio, contexts, advance } = fixture();
  audio.setEnabled(true);
  audio.unlock();
  assert(audio.play("turn"));
  advance(80);
  assert.equal(audio.play("tap"), false);
  assert.equal(audio.play("select"), false);
  assert.equal(audio.play("turn"), false);
  advance(200);
  assert(audio.play("select"));
  assert.equal(audio.play("select"), false);
  advance(1000);
  assert(audio.play("win"));
  advance(70);
  assert.equal(audio.play("play"), false);
  assert.equal(contexts.length, 1);
  assert.equal(contexts[0].plays.length, 3);
});

test("immediate server feedback preempts a tap without allocating another context", () => {
  const { audio, contexts, advance } = fixture();
  audio.setEnabled(true);
  audio.unlock();
  assert(audio.play("tap"));
  advance(10);
  assert(audio.play("play"));
  advance(10);
  assert(audio.play("turn"));
  advance(10);
  assert(audio.play("win"));
  advance(10);
  assert.equal(audio.play("tap"), false);
  assert.equal(contexts.length, 1);
  assert.deepEqual(contexts[0].plays, [
    "assets/audio/tap.wav",
    "assets/audio/play.wav",
    "assets/audio/turn.wav",
    "assets/audio/win.wav",
  ]);
});
test("hide stops current sound; show never catches up; disabling and destroy are final", () => {
  const { audio, contexts, advance } = fixture();
  audio.setEnabled(true);
  audio.unlock();
  audio.play("win");
  const context = contexts[0],
    before = context.stops;
  audio.suspend();
  assert(context.stops > before);
  advance(2000);
  assert.equal(audio.play("turn"), false);
  audio.resume();
  assert.equal(context.plays.length, 1);
  assert(audio.play("turn"));
  audio.setEnabled(false);
  advance(1000);
  assert.equal(audio.play("play"), false);
  audio.destroy();
  audio.destroy();
  audio.setEnabled(true);
  audio.unlock();
  audio.resume();
  assert.equal(audio.play("tap"), false);
  assert.equal(context.destroys, 1);
  assert.equal(contexts.length, 1);
});
test("unsupported, throwing and asynchronously failing audio never throws or recreates loops", () => {
  for (const host of [
    {},
    {
      createInnerAudioContext() {
        throw Error("no audio");
      },
    },
  ]) {
    const audio = new GameAudio(host);
    assert.doesNotThrow(() => {
      audio.setEnabled(true);
      audio.unlock();
      audio.play("tap");
      audio.suspend();
      audio.resume();
      audio.destroy();
    });
  }
  const { audio, contexts, advance } = fixture();
  audio.setEnabled(true);
  audio.unlock();
  contexts[0].error();
  advance(1000);
  assert.equal(audio.play("win"), false);
  audio.unlock();
  assert.equal(contexts.length, 1);
  assert.equal(contexts[0].destroys, 1);
  const bad = new GameAudio({
    createInnerAudioContext() {
      return {
        set autoplay(v) {
          throw Error("property unavailable");
        },
        destroy() {},
      };
    },
  });
  assert.doesNotThrow(() => {
    bad.setEnabled(true);
    bad.unlock();
    bad.play("tap");
    bad.destroy();
  });
});
test("original sound assets are short mono PCM WAV files with gentle levels", () => {
  for (const name of ["tap", "select", "play", "turn", "win"]) {
    const wav = readFileSync(
      new URL(`../assets/audio/${name}.wav`, import.meta.url),
    );
    assert.equal(wav.toString("ascii", 0, 4), "RIFF");
    assert.equal(wav.toString("ascii", 8, 12), "WAVE");
    assert.equal(wav.readUInt32LE(4), wav.length - 8);
    assert.equal(wav.readUInt16LE(20), 1);
    assert.equal(wav.readUInt16LE(22), 1);
    assert.equal(wav.readUInt16LE(34), 16);
    const duration = (wav.length - 44) / (wav.readUInt32LE(24) * 2);
    assert(duration > 0 && duration < 0.5);
    let peak = 0;
    for (let i = 44; i < wav.length; i += 2)
      peak = Math.max(peak, Math.abs(wav.readInt16LE(i)));
    assert(peak > 0 && peak < 9000);
    assert.equal(wav.readInt16LE(44), 0);
  }
});
