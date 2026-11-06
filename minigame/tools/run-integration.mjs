import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
const script = fileURLToPath(
  new URL("../tests/full-round.mjs", import.meta.url),
);
for (const [name, call, pairs] of [
  ["hard-trump singles", "0", "0"],
  ["called-trump singles", "1", "0"],
  ["called-trump pairs", "1", "1"],
]) {
  let passed = false;
  for (let attempt = 1; attempt <= 5; attempt++) {
    const result = spawnSync(process.execPath, [script], {
      encoding: "utf8",
      timeout: 60000,
      env: {
        ...process.env,
        SHENGJI_TEST_CALL: call,
        SHENGJI_TEST_PAIRS: pairs,
      },
    });
    process.stdout.write(result.stdout || "");
    if (result.status === 0) {
      passed = true;
      break;
    }
    // Only a genuinely uncallable random deal may be redealt. Never hide a bug.
    if (
      call === "1" &&
      (result.stderr || "").includes("Random deal has no callable hand")
    ) {
      console.log(`${name}: no callable hand, redeal ${attempt}/5`);
      continue;
    }
    process.stderr.write(
      result.stderr || String(result.error || "Integration failed"),
    );
    process.exit(1);
  }
  if (!passed) throw Error(`${name}: no callable deal after five attempts`);
  console.log(`PASS mode: ${name}`);
}
