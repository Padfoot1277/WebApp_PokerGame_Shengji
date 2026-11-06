<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useGameStore } from "../store/game";
import { teamName, relativeSeat } from "../utils/seat";
const game = useGameStore();
const confirm = ref(false);
const copied = ref(false);
watch(
  () => `${game.view?.roomId}:${game.view?.roundIndex}:${game.view?.phase}`,
  () => {
    confirm.value = false;
    copied.value = false;
  },
);
const sound = ref(false);
const vibration = ref(false);
let audio: AudioContext | null = null;
function enableSound() {
  if (sound.value) {
    audio ??= new AudioContext();
    void audio.resume();
  }
}
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
  if (!confirm.value && !voted.value) {
    confirm.value = true;
    return;
  }
  game.sendEvent(voted.value ? "game.cancel_surrender" : "game.surrender", {});
  confirm.value = false;
}
async function invite() {
  if (!v.value) return;
  const url = new URL(location.href);
  url.searchParams.set("room", v.value.roomId);
  try {
    await navigator.clipboard.writeText(url.toString());
    copied.value = true;
  } catch {
    game.pushMessage("notice", `请分享房间码：${v.value.roomId}`);
  }
}
</script>
<template>
  <div class="connection-banner" role="status">
    <span :class="{ offline: game.wsStatus !== 'open' }">{{
      {
        idle: "尚未连接",
        connecting: "正在连接，恢复牌局…",
        open: "已连接",
        closed: "连接中断，正在重连…",
      }[game.wsStatus]
    }}</span
    ><template v-if="v"
      ><span>房间 {{ v.roomId }}</span
      ><button
        @click="
          game.disconnect();
          game.view = null;
        "
      >
        返回大厅</button
      ><button @click="invite">
        {{ copied ? "邀请链接已复制" : "邀请朋友" }}
      </button></template
    ><button
      v-if="game.wsStatus === 'connecting' || game.wsStatus === 'closed'"
      @click="game.disconnect()"
    >
      停止重连</button
    ><label
      ><input v-model="sound" type="checkbox" @change="enableSound" />
      提示音</label
    ><label><input v-model="vibration" type="checkbox" /> 振动</label>
  </div>
  <section
    v-if="v && (v.phase === 'round_settle' || v.phase === 'game_over')"
    class="panel settlement"
    aria-live="polite"
  >
    <h2>
      {{
        v.phase === "game_over"
          ? `${teamName(v.winnerTeam)}升到 A，获胜！`
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
  <details v-if="canSurrender" class="surrender">
    <summary>提前结束本局 / 弃局</summary>
    <p v-if="v">
      {{
        v.surrenderVotes.filter(Boolean).length
          ? "已有牌友申请弃局。同队两人确认后立即按当前得分结算。"
          : "需要提前结束本局？可与队友共同申请弃局。"
      }}
    </p>
    <button
      :disabled="game.pending || game.wsStatus !== 'open'"
      @click="surrender"
    >
      {{ voted ? "撤回弃局" : confirm ? "确认申请弃局" : "申请弃局" }}</button
    ><button v-if="confirm" @click="confirm = false">取消</button>
  </details>
</template>
<style scoped>
.connection-banner {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
  margin: 12px auto;
  max-width: 1100px;
  padding: 0 12px;
  color: #26765c;
  font-size: 13px;
}
.connection-banner button {
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
.surrender {
  margin: 0 auto;
  width: 100%;
  max-width: 1100px;
  padding: 0 12px;
  color: #6c786e;
  font-size: 13px;
}
.surrender button {
  background: #f1e6df;
  color: #875844;
  margin-right: 8px;
}
</style>
