<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import TrickPlayView from "./TrickPlayView.vue";
import SeatIdentity from "./SeatIdentity.vue";
import { showingLast, moveTag, seatOffsets, leaderSeats } from "../utils/table";
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
const positions = seatOffsets;
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
const leaders = computed(() =>
  v.value ? leaderSeats(v.value, previous.value) : [],
);
const biggest = computed(() =>
  last.value ? v.value?.trick.winnerSeat : v.value?.trick.biggerSeat,
);
const seatTags = computed(() =>
  [0, 1, 2, 3].map((index) => {
    const tags = [];
    if (index === turn.value && !previous.value)
      tags.push({ key: "turn", text: "⏳", label: "轮到", tone: "turn" });
    if (leaders.value.includes(index))
      tags.push({ key: "leader", text: "🚩", label: "先手" });
    if (moves.value?.[index] && index === biggest.value)
      tags.push({ key: "biggest", text: "👑", label: "最大", tone: "biggest" });
    const move = moveTag(moves.value?.[index]);
    if (move) tags.push({ key: "move", text: move.label, label: move.label });
    if (!v.value?.seats[index]?.online)
      tags.push({ key: "offline", text: "📴 离线", label: "离线" });
    if (v.value?.surrenderVotes[index])
      tags.push({ key: "surrender", text: "🏳️ 申请弃局", label: "申请弃局" });
    return tags;
  }),
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
            : `牌桌（打家得分：${v.points}）`
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
          {
            active: !previous && index === turn,
            mine: index === v.mySeat,
            biggest: !!moves?.[index] && index === biggest,
          },
        ]"
      >
        <header>
          <SeatIdentity
            :name="v.seats[index]?.name || '空位'"
            :tags="seatTags[index] ?? []"
          />
        </header>
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
.seat.biggest {
  background: #e2d3ff;
  border-color: #9062dd;
  box-shadow: inset 0 0 0 1px #b38ae8;
}
.mine {
  box-shadow: inset 0 -3px #21846b;
}
header {
  margin-bottom: 6px;
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
  .table-heading {
    margin-bottom: 8px;
    font-size: 12px;
  }
}
</style>
