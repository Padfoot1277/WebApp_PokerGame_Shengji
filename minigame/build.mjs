import { fileURLToPath } from "node:url";
import { build } from "esbuild";
import { cp, mkdir, writeFile } from "node:fs/promises";
await mkdir(new URL("./dist/", import.meta.url), { recursive: true });
await cp(
  new URL("./assets/", import.meta.url),
  new URL("./dist/assets/", import.meta.url),
  { recursive: true },
);
await build({
  entryPoints: [fileURLToPath(new URL("./src/main.ts", import.meta.url))],
  outfile: fileURLToPath(new URL("./dist/game.js", import.meta.url)),
  bundle: true,
  format: "iife",
  target: "es2020",
  platform: "neutral",
});
await writeFile(
  new URL("./dist/game.json", import.meta.url),
  JSON.stringify(
    {
      deviceOrientation: "landscape",
      networkTimeout: { connectSocket: 10000 },
    },
    null,
    2,
  ),
);
