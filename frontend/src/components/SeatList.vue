<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import { seatName } from "../utils/seat";
const game = useGameStore();
const seats = computed(() => game.view?.seats ?? []);
const mySeat = computed(() => game.view?.mySeat ?? -1);
const enabled = computed(() => game.wsStatus === "open" && !game.pending);
</script>
<template>
  <section class="panel">
    <h3>选择座位 <small>东与西、南与北互为队友</small></h3>
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
        <p>{{ seat.name || "等待牌友入座" }}</p>
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
</style>
