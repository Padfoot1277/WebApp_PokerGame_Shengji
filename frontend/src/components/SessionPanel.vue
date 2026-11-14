<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from "vue";
import { useGameStore } from "../store/game";
import { teamName, relativeSeat } from "../utils/seat";
const game = useGameStore();
const sound = ref(localStorage.getItem("shengji-sound") !== "false");
const vibration = ref(localStorage.getItem("shengji-vibration") !== "false");
let audio: AudioContext | null = null;
function enableSound() {
  if (sound.value) {
    try {
      audio ??= new AudioContext();
      void audio.resume().catch(() => {});
    } catch {
      /* Optional browser audio support. */
    }
  }
}
onMounted(() => window.addEventListener("pointerdown", enableSound));
onUnmounted(() => window.removeEventListener("pointerdown", enableSound));
watch(sound, (value) => localStorage.setItem("shengji-sound", String(value)));
watch(vibration, (value) =>
  localStorage.setItem("shengji-vibration", String(value)),
);
let lastTurn = "";
watch(
  () => game.view?.version,
  () => {
    const s = game.view;
    if (!s || s.mySeat < 0 || game.wsStatus !== "open") return;
    const mine =
      s.phase === "play_trick"
        ? s.trick.turnSeat
        : s.phase === "trump_fight"
          ? s.fightTurnSeat
          : s.phase === "bottom"
            ? s.bottomOwnerSeat
            : s.phase === "call_trump"
              ? s.callTurnSeat
              : -1;
    const key = `${s.roomId}:${s.roundIndex}:${s.trickIndex}:${s.phase}:${mine}`;
    if (key === lastTurn) return;
    lastTurn = key;
    if (mine !== s.mySeat) return;
    if (vibration.value) navigator.vibrate?.(80);
    if (sound.value && audio) {
      const tone = audio.createOscillator();
      const gain = audio.createGain();
      tone.connect(gain);
      gain.connect(audio.destination);
      tone.frequency.value = 660;
      gain.gain.value = 0.04;
      tone.start();
      tone.stop(audio.currentTime + 0.12);
    }
  },
);
watch(
  () => game.view?.phase,
  (phase) => {
    if (phase === "round_settle" || phase === "game_over")
      window.scrollTo({ top: 0 });
  },
);
const bottomResult = computed(() => {
  const s = game.view;
  if (!s) return "";
  if (s.trick.winnerSeat % 2 === s.starterSeat % 2)
    return "坐家赢得末墩，打家未能抠底";
  if (s.bottomOwnerSeat < 0) return "无人定主，本局不抠底";
  if (!s.trump.hasTrumpSuit && s.bottomOwnerSeat % 2 !== s.starterSeat % 2)
    return "打家攻主，本局不抠底";
  return `打家抠底：${s.bottomPoints} 分 × ${s.bottomMul}，实际获得 ${s.bottomAward} 分`;
});
const v = computed(() => game.view);
const canSurrender = computed(
  () =>
    v.value &&
    ["bottom", "trump_fight", "play_trick"].includes(v.value.phase) &&
    v.value.mySeat >= 0,
);
const voted = computed(() => {
  const s = v.value;
  return s?.surrenderVotes[s.mySeat] ?? false;
});
function surrender() {
  game.sendEvent(voted.value ? "game.cancel_surrender" : "game.surrender", {});
}
function leaveRoom() {
  const s = v.value;
  if (
    s &&
    s.mySeat >= 0 &&
    !["lobby", "round_settle", "game_over"].includes(s.phase) &&
    !window.confirm(
      "返回大厅会断开当前连接，座位仍保留。重新加入原房间可以继续。确定返回吗？",
    )
  )
    return;
  game.disconnect();
  game.view = null;
}
</script>
<template>
  <div class="connection-banner">
    <nav class="session-actions" aria-label="房间操作">
      <button v-if="v" @click="leaveRoom">返回大厅</button>
      <a class="archive-link" href="#/archives">小局档案</a>
      <button
        v-if="canSurrender"
        :disabled="game.pending || game.wsStatus !== 'open'"
        :aria-pressed="voted"
        @click="surrender"
      >
        {{ voted ? "取消弃局" : "申请弃局" }}
      </button>
    </nav>
    <span role="status" :class="{ offline: game.wsStatus !== 'open' }"
      >{{
        {
          idle: "尚未连接",
          connecting: "正在连接，恢复牌局…",
          open: "已连接",
          closed: "连接中断，正在重连…",
        }[game.wsStatus]
      }}<template v-if="v">，房间 {{ v.roomId }}</template></span
    >
    <button
      v-if="game.wsStatus === 'connecting' || game.wsStatus === 'closed'"
      @click="game.disconnect()"
    >
      停止重连
    </button>
  </div>
  <section
    v-if="v && (v.phase === 'round_settle' || v.phase === 'game_over')"
    class="panel settlement"
    aria-live="polite"
  >
    <h2>
      {{
        v.phase === "game_over"
          ? `${v.mySeat < 0 ? teamName(v.winnerTeam) : v.winnerTeam === v.mySeat % 2 ? "我方" : "对方"}升到 A，获胜！`
          : "小局结算"
      }}
    </h2>
    <p v-if="v.surrendered">
      本局由同队两人确认弃局，未完成的墩不计分，不抠底。
    </p>
    <p>
      打家得分 <strong>{{ v.roundPointsFinal }}</strong> ·
      {{ v.roundResultLabel }}
    </p>
    <p>坐家 +{{ v.callerDelta }} 级 · 打家 +{{ v.defenderDelta }} 级</p>
    <p v-if="v.bottomRevealed">
      {{ bottomResult }}
    </p>
    <p v-if="v.phase === 'round_settle'">
      {{ relativeSeat(v.nextStarterSeat, v.mySeat) }}可以开始下一小局。
    </p>
  </section>
</template>
<style scoped>
.connection-banner {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  margin: 12px auto;
  width: 100%;
  max-width: 1100px;
  padding: 0 12px;
  color: #26765c;
  font-size: 13px;
}
.session-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.archive-link {
  display: inline-flex;
  align-items: center;
  min-height: 42px;
  border-radius: 8px;
  background: #23785f;
  color: #fff;
  text-decoration: none;
}
.archive-link:hover {
  filter: brightness(1.07);
}
.archive-link:focus-visible {
  outline: 3px solid #cb9e2e;
  outline-offset: 3px;
}
.connection-banner button,
.archive-link {
  padding: 6px 10px;
  font-size: 12px;
}
.offline {
  color: #a4601a;
}
.settlement {
  border: 2px solid #b99a45;
  margin-bottom: 16px;
}
.settlement strong {
  font-size: 26px;
}
</style>
