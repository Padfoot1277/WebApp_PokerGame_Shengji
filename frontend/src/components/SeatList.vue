<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import { seatName } from "../utils/seat";
const game = useGameStore();
const seats = computed(() => game.view?.seats ?? []);
const mySeat = computed(() => game.view?.mySeat ?? -1);
const enabled = computed(() => game.wsStatus === "open" && !game.pending);
const botCount = computed(() => seats.value.filter((s) => s.isBot).length);
const occupied = computed(() => seats.value.filter((s) => s.uid).length);
const ready = computed(
  () => seats.value.filter((s) => s.uid && s.online && s.ready).length,
);
</script>
<template>
  <section class="panel">
    <h3>选择座位 <small>坐在对面的牌友是你的队友</small></h3>
    <p v-if="seats.length" class="readiness" role="status">
      已入座 {{ occupied }} / 4 · 已准备 {{ ready }} / 4。
      {{
        mySeat < 0
          ? occupied === 4
            ? "座位已满，你正在观战。"
            : "点击空位入座，选择与朋友搭档的位置。"
          : seats[mySeat]?.ready
            ? "你已准备，等待其余牌友。"
            : "入座后点击准备，四人准备即开局。"
      }}
    </p>
    <div v-if="game.view?.phase === 'lobby'" class="bot-controls">
      <button
        :disabled="!enabled || occupied === 4"
        @click="game.sendEvent('room.add_bot', {})"
      >
        增加陪练
      </button>
      <button
        :disabled="!enabled || botCount === 0"
        @click="game.sendEvent('room.remove_bot', {})"
      >
        减少陪练
      </button>
      <small>每次一位，按添加顺序移除；陪练自动准备及出牌。</small>
    </div>
    <div class="seat-grid">
      <div
        v-for="(seat, index) in seats"
        :key="index"
        class="seat"
        :class="{ mine: index === mySeat }"
      >
        <strong
          >{{ seatName(index) }} {{ index === mySeat ? "· 我" : "" }}</strong
        >
        <p>
          {{ seat.name || "等待牌友入座" }}{{ seat.isBot ? " · 陪练" : "" }}
        </p>
        <small>{{
          seat.uid
            ? seat.online
              ? seat.ready
                ? "已准备"
                : "未准备"
              : "暂时离线"
            : "空位"
        }}</small>
        <div class="actions">
          <button
            v-if="!seat.uid"
            :disabled="!enabled"
            @click="game.sendEvent('room.sit', { seat: index })"
          >
            入座
          </button>
          <template v-if="index === mySeat"
            ><button
              :disabled="!enabled"
              @click="
                game.sendEvent(seat.ready ? 'room.unready' : 'room.ready', {})
              "
            >
              {{ seat.ready ? "取消准备" : "准备" }}</button
            ><button
              :disabled="!enabled"
              @click="game.sendEvent('room.leave_seat', {})"
            >
              离座
            </button></template
          >
        </div>
      </div>
    </div>
    <p v-if="!seats.length">加入房间后选择座位，四人准备完成自动开始。</p>
  </section>
</template>
<style scoped>
.bot-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}
.seat-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.seat {
  padding: 16px;
  background: #f2f7f4;
  border-radius: 12px;
}
.mine {
  outline: 2px solid #20856d;
}
.actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
small {
  font-weight: normal;
  color: #65796f;
}
.readiness {
  margin: 10px 0 14px;
  padding: 10px 12px;
  border-radius: 8px;
  background: #e7f0f2;
  color: #315568;
  font-size: 14px;
}
</style>
