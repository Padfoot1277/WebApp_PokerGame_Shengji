<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import { relativeSeat } from "../utils/seat";
import TrickPlayView from "./TrickPlayView.vue";
import { showingLast, moveTag } from "../utils/table";
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
const last = computed(() =>
  v.value ? showingLast(v.value, previous.value) : false,
);
const moves = computed(() =>
  last.value ? v.value?.trick.lastMoves : v.value?.trick.playedMoves,
);
const leader = computed(() =>
  last.value ? v.value?.trick.lastLeaderSeat : v.value?.trick.leaderSeat,
);
const biggest = computed(() =>
  last.value ? v.value?.trick.winnerSeat : v.value?.trick.biggerSeat,
);
</script>
<template>
  <section v-if="v" class="panel">
    <div class="table-heading">
      <strong>{{
        previous
          ? "上一墩 · 历史出牌"
          : ["round_settle", "game_over"].includes(v.phase)
            ? "末墩出牌 · 本局已结束"
            : `本墩牌桌（打家得分：${v.points}）`
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
          { active: !previous && index === turn, mine: index === v.mySeat },
        ]"
      >
        <header>
          <strong>{{ relativeSeat(index, v.mySeat) }}</strong>
          <small class="nickname">{{ v.seats[index]?.name || "空位" }}</small>
        </header>
        <div class="seat-meta">
          <span v-if="index === turn && !previous" class="chip turn"
            >⏳ 轮到</span
          >
          <span v-if="index === leader" class="chip">🚩 先手</span>
          <span v-if="moves?.[index] && index === biggest" class="chip biggest"
            >👑 最大</span
          >
          <span v-if="moveTag(moves?.[index])" class="chip">{{
            moveTag(moves?.[index])?.label
          }}</span>
          <span v-if="!v.seats[index]?.online" class="chip">📴 离线</span>
          <span v-if="v.surrenderVotes[index]" class="chip">🏳️ 申请弃局</span>
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
@media (max-width: 759px) {
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
    font-size: 11px;
    gap: 4px;
  }
  .table-heading {
    margin-bottom: 8px;
    font-size: 12px;
  }
  .chip {
    padding: 2px 4px;
  }
}
.nickname {
  margin-left: auto;
  overflow-wrap: anywhere;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 7px;
  border-radius: 8px;
  background: #fff;
  line-height: 1.3;
}
.chip.turn {
  background: #fff0c4;
  color: #79531b;
}
.chip.biggest {
  background: #e7dcff;
  color: #6043a0;
}
</style>
