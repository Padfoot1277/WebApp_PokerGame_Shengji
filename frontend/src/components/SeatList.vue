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

const myReady = computed(() =>
    mySeat.value >= 0 ? game.view.seats[mySeat.value].ready : false
)

function ready() {
  game.sendEvent('room.ready', {})
}

function unready() {
  game.sendEvent('room.unready', {})
}

function seatLabel(idx: number): string {
  const map = ['⓪', '①', '②', '③']
  return map[idx] ?? String(idx)
}
</script>

<template>
  <div class="panel">
    <h3>座位信息</h3>

    <div class="seat-grid">
      <div
          v-for="(s, displayIdx) in displaySeats"
          :key="seatOrder[displayIdx]"
          class="seat"
          :class="{ mine: seatOrder[displayIdx] === mySeat }"
      >
        <!-- 显示真实 seat 编号 -->
        <div><strong>{{ seatLabel(seatOrder[displayIdx]) }}号位 </strong></div>

        <div v-if="s.uid">昵称：{{ s.uid }}</div>
        <div v-else>空位</div>
        <div :class="s.online ? 'status-online' : 'status-offline'">
          状态：{{ s.online ? '在线' : '离线' }}
        </div>
        <div>准备：{{ s.ready ? '是' : '否' }}</div>

        <!-- 按钮区域 -->
        <div class="actions">
          <button
              v-if="!s.uid"
              @click="sit(seatOrder[displayIdx])"
          >
            入座
          </button>

          <button
              v-if="seatOrder[displayIdx] === mySeat"
              @click="leave"
          >
            离座
          </button>

          <button
              v-if="mySeat >= 0 && !myReady && seatOrder[displayIdx] === mySeat"
              @click="ready"
          >
            准备
          </button>

          <button
              v-if="mySeat >= 0 && myReady && seatOrder[displayIdx] === mySeat"
              @click="unready"
          >
            取消准备
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>

/* 标题层级 */
.panel h3 {
  margin: 0 0 14px 0;
  font-size: 18px;
  font-weight: 600;
}

/* 保持 2x2 布局 */
.seat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

/* 单个座位块 */
.seat {
  background: #c7edcc;
  padding: 14px 14px 16px 14px;
  border-radius: var(--radius);
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  min-height: 150px;
}

/* 自己的座位高亮 */
.seat.mine {
  outline: 2px solid #4da3ff;
}

/* Seat 标题 */
.seat strong {
  font-size: 18px;
  font-weight: 600;
}

/* 文字信息层级 */
.seat div {
  font-size: 16px;
  line-height: 1.4;
}

/* 状态文本稍微淡一点 */
.seat div:nth-child(n+2) {
  color: rgba(0, 0, 0, 0.75);
}

/* 按钮统一尺寸 */
.panel button {
  margin-top: 6px;
  height: 38px;
  min-width: 70px;
  padding: 0 14px;
  border-radius: 6px;
  background: #4da3ff;
  color: #fff;
  align-self: flex-start;
  font-size: 16px;
  font-weight: 500;
}

/* hover 保持 */
@media (hover: hover) and (pointer: fine) {
  .panel button:hover {
    filter: brightness(1.1);
  }
}
.status-online {
  color: #2e7d32;
}

.status-offline {
  color: #b71c1c;
}

.actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
  flex-wrap: wrap; /* 防止窄屏溢出 */
}

</style>
