<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import { relativeSeat, seatName, teamName } from "../utils/seat";
import TrickPlayView from "./TrickPlayView.vue";
const game = useGameStore();
const previous = computed({
  get: () => game.reviewPrevious,
  set: (value) => {
    game.reviewPrevious = value;
  },
});
const v = computed(() => game.view);
const canReview = computed(
  () => v.value?.trick.lastMoves.some(Boolean) ?? false,
);
const positions = [2, 3, 1, 0];
const order = computed(() =>
  positions.map(
    (offset) => (Math.max(0, v.value?.mySeat ?? 0) + offset + 4) % 4,
  ),
);
const turn = computed(() => {
  const s = v.value;
  if (!s) return -1;
  switch (s.phase) {
    case "call_trump":
      return s.callTurnSeat;
    case "bottom":
      return s.bottomOwnerSeat;
    case "trump_fight":
      return s.fightTurnSeat;
    case "play_trick":
      return s.trick.turnSeat;
    default:
      return -1;
  }
});
const moves = computed(() => {
  const t = v.value?.trick;
  return previous.value ||
    ["round_settle", "game_over"].includes(v.value?.phase ?? "")
    ? t?.lastMoves
    : t?.playedMoves;
});
</script>
<template>
  <section v-if="v" class="panel">
    <div class="table-heading">
      <strong>{{
        previous
          ? "上一墩 · 历史出牌"
          : ["round_settle", "game_over"].includes(v.phase)
            ? "末墩出牌 · 本局已结束"
            : "本墩牌桌"
      }}</strong
      ><label
        ><input
          v-model="previous"
          type="checkbox"
          :disabled="!canReview && !previous"
        />
        查看上一墩</label
      >
    </div>
    <p v-if="previous" class="history-note" role="status">
      正在回放上一墩，选牌和出牌已暂停。取消“查看上一墩”即可继续。
    </p>
    <div class="table">
      <article
        v-for="(index, position) in order"
        :key="index"
        class="seat"
        :class="[
          'position-' + position,
          { active: index === turn, mine: index === v.mySeat },
        ]"
      >
        <header>
          <strong>{{ relativeSeat(index, v.mySeat) }}</strong>
          <small>{{
            v.mySeat < 0
              ? teamName(index % 2)
              : `${seatName(index)} · ${index % 2 === v.mySeat % 2 ? "我方" : "对方"}`
          }}</small>
        </header>
        <div>
          {{ v.seats[index]?.name || "空位" }}
          <small>{{ v.seats[index]?.online ? "" : "· 离线" }}</small>
        </div>
        <div class="seat-meta">
          <span>{{ v.seats[index]?.handCount }} 张</span
          ><span v-if="index === turn">轮到操作</span
          ><span v-if="v.phase !== 'call_trump' && index === v.starterSeat"
            >坐家</span
          ><span v-if="v.phase === 'play_trick' && index === v.trick.biggerSeat"
            >当前最大</span
          ><span v-if="v.surrenderVotes[index]">申请弃局</span>
        </div>
        <TrickPlayView
          v-if="['play_trick', 'round_settle', 'game_over'].includes(v.phase)"
          :move="moves?.[index] ?? null"
        />
      </article>
    </div>
  </section>
</template>
<style scoped>
.table-heading {
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
  font-size: 14px;
}
.table {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.seat {
  min-width: 0;
  background: #edf5ef;
  padding: 8px;
  border: 2px solid transparent;
  border-radius: 12px;
}
.position-0,
.position-3 {
  grid-column: 1/-1;
  width: 90%;
  justify-self: center;
}
@keyframes turn-in {
  from {
    box-shadow: 0 0 0 5px #b37e1430;
  }
  to {
    box-shadow: 0 0 0 0 transparent;
  }
}
.active {
  animation: turn-in 0.4s ease-out;
  border-color: #b37e14;
  background: #fff8dc;
}
.mine {
  box-shadow: inset 0 -3px #21846b;
}
.seat-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12px;
  color: #466557;
  margin-top: 8px;
}
small {
  color: #65796f;
}
header {
  margin-bottom: 6px;
}
header {
  display: flex;
  gap: 8px;
  align-items: baseline;
  flex-wrap: wrap;
}
.seat-meta {
  margin-top: 4px;
  gap: 6px;
}
.table-heading {
  gap: 8px;
  flex-wrap: wrap;
}
.table-heading label {
  font-size: 12px;
}
.history-note {
  padding: 10px;
  margin-bottom: 10px;
  background: #e5eff3;
  border-radius: 8px;
  color: #315568;
  font-size: 13px;
}
@media (max-width: 480px) {
  .position-0,
  .position-3 {
    grid-column: auto;
    width: 100%;
  }
  .seat {
    padding: 6px;
  }
  .seat header small {
    font-size: 10px;
  }
  .seat-meta {
    font-size: 10px;
  }
}
</style>
