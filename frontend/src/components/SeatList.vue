<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

// 原始数据
const seats = computed(() => game.view?.seats ?? [])
const mySeat = computed(() => game.view?.mySeat ?? -1)

// UI 展示顺序映射：0 1 3 2
const seatOrder = [0, 1, 3, 2]

// 根据映射得到展示用 seats
const displaySeats = computed(() => {
  const arr = seats.value
  return seatOrder
      .map(i => arr[i])
      .filter(Boolean)
})

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
          v-for="(s, displayIdx) in displaySeats"
          :key="seatOrder[displayIdx]"
          class="seat"
          :class="{ mine: seatOrder[displayIdx] === mySeat }"
      >
        <!-- 显示真实 seat 编号 -->
        <div>Seat {{ seatOrder[displayIdx] }}</div>

        <div v-if="s.uid">UID: {{ s.uid }}</div>
        <div v-else>空位</div>

        <div>状态：{{ s.online ? '在线' : '离线' }}</div>
        <div>准备：{{ s.ready ? '是' : '否' }}</div>

        <!-- 入座：传真实 seat index -->
        <button
            v-if="!s.uid"
            @click="sit(seatOrder[displayIdx])"
        >
          入座
        </button>

        <!-- 离座：判断真实 seat index -->
        <button
            v-if="seatOrder[displayIdx] === mySeat"
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
