<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const seats = computed(() => game.view?.seats ?? [])
const mySeat = computed(() => game.view?.mySeat ?? -1)

function sit(seat: number) {
  game.sendEvent('room.sit', { seat })
}

function leave() {
  game.sendEvent('room.leave_seat', {})
}
</script>

<template>
  <div class="panel">
    <h3>座位</h3>

    <div class="seat-grid">
      <div
          v-for="(s, idx) in seats"
          :key="idx"
          class="seat"
          :class="{ mine: idx === mySeat }"
      >
        <div>Seat {{ idx }}</div>
        <div v-if="s.uid">UID: {{ s.uid }}</div>
        <div v-else>空位</div>
        <div>状态：{{ s.online ? '在线' : '离线' }}</div>
        <div>准备：{{ s.ready ? '是' : '否' }}</div>

        <button
            v-if="!s.uid"
            @click="sit(idx)"
        >
          入座
        </button>

        <button
            v-if="idx === mySeat"
            @click="leave"
        >
          离座
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.seat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}

.seat {
  background: var(--bg-card);
  padding: 8px;
  border-radius: var(--radius);
}

.seat.mine {
  outline: 2px solid #4da3ff;
}
</style>

