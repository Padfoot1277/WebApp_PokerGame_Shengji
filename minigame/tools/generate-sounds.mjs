// Original procedural sound design: sine partials with smooth attack/release.
// Deterministic, no sampled recordings, third-party assets or network downloads.
import { mkdirSync, writeFileSync } from "node:fs";
const directory = new URL("../assets/audio/", import.meta.url);
mkdirSync(directory, { recursive: true });
const sampleRate = 22050;
const specs = {
  tap: { seconds: 0.045, notes: [420] },
  select: { seconds: 0.065, notes: [620] },
  play: { seconds: 0.12, notes: [330, 440] },
  turn: { seconds: 0.22, notes: [523.25, 659.25] },
  win: { seconds: 0.42, notes: [523.25, 659.25, 783.99] },
};
for (const [name, { seconds, notes }] of Object.entries(specs)) {
  const frames = Math.round(seconds * sampleRate);
  const pcm = Buffer.alloc(frames * 2);
  for (let i = 0; i < frames; i++) {
    const time = i / sampleRate;
    const position = time / seconds;
    const noteIndex = Math.min(
      notes.length - 1,
      Math.floor(position * notes.length),
    );
    const local = position * notes.length - noteIndex;
    const envelope = Math.sin(Math.PI * local) ** 2;
    const fundamental = notes[noteIndex];
    const wave =
      Math.sin(2 * Math.PI * fundamental * time) * 0.88 +
      Math.sin(4 * Math.PI * fundamental * time) * 0.12;
    pcm.writeInt16LE(Math.round(32767 * 0.22 * envelope * wave), i * 2);
  }
  const header = Buffer.alloc(44);
  header.write("RIFF", 0);
  header.writeUInt32LE(36 + pcm.length, 4);
  header.write("WAVEfmt ", 8);
  header.writeUInt32LE(16, 16);
  header.writeUInt16LE(1, 20);
  header.writeUInt16LE(1, 22);
  header.writeUInt32LE(sampleRate, 24);
  header.writeUInt32LE(sampleRate * 2, 28);
  header.writeUInt16LE(2, 32);
  header.writeUInt16LE(16, 34);
  header.write("data", 36);
  header.writeUInt32LE(pcm.length, 40);
  const output = Buffer.concat([header, pcm]);
  writeFileSync(new URL(`${name}.wav`, directory), output);
  console.log(
    `${name}.wav ${output.length} bytes / ${Math.round(seconds * 1000)} ms`,
  );
}
