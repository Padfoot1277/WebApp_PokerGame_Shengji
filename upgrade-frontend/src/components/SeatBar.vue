<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'
import TrickPlayView from './TrickPlayView.vue'

const game = useGameStore()
const v = computed(() => game.view)

const seats = computed(() => v.value?.seats ?? [])
const trick = computed(() => v.value?.trick)

function seatStatus(idx: number): string {
  const view = v.value
  if (!view) return ''

  const s = view.seats[idx]
  if (!s.uid) return '空位'
  if (!s.online) return '离线'
  if (view.phase === 'lobby') return s.ready ? '已准备' : '未准备'

  // call_trump：轮到谁/谁已跳过
  if (view.phase === 'call_trump') {
    if (view.callPassedSeats[idx]) return '已跳过'
    if (idx === view.callTurnSeat && view.callMode === 'ordered') return '定主中（轮到）'
    if (view.callMode === 'race') return '抢主中'
    return '等待定主'
  }

  // bottom：坐家扣底
  if (view.phase === 'bottom') {
    return idx === view.bottomOwnerSeat ? '扣底中（坐家）' : '等待扣底'
  }

  // trump_fight：非坐家改/攻主窗口
  if (view.phase === 'trump_fight') {
    if (idx === view.bottomOwnerSeat) return '坐家（等待改/攻主结束）'
    if (view.fightPassedSeats[idx]) return '已跳过'
    return '改/攻主窗口'
  }

  // play_trick / follow_trick：出牌/跟牌
  if (view.phase === 'play_trick' || view.phase === 'follow_trick') {
    const pm = view.trick?.playedMoves?.[idx]
    const hasPlayed = !!pm // 你的快照里未出牌是 null
    if (hasPlayed) return idx === view.trick.leaderSeat ? '已出牌（先手）' : '已出牌'
    if (idx === view.trick.turnSeat) return '出牌中（轮到）'
    return '等待出牌'
  }

  return ''
}

</script>

<template>
  <div class="seat-bar">
    <div
        v-for="(s, idx) in seats"
        :key="idx"
        class="seat"
    >
      <div class="seat-head">
        <strong>Seat {{ idx }}</strong>
        <span v-if="idx === v.mySeat">（我）</span>
      </div>
      <div class="status">
        状态：<span class="badge">{{ seatStatus(idx) }}</span>
      </div>
      <div>UID: {{ s.uid || '空' }}</div>
      <div>手牌数：{{ s.handCount }}</div>
      <div>在线：{{ s.online ? '是' : '否' }}</div>

      <TrickPlayView
          :move="trick?.playedMoves?.[idx] ?? null"
          :is-leader="idx === trick?.leaderSeat"
          :is-turn="idx === trick?.turnSeat"
      />
    </div>
  </div>
</template>

<style scoped>
.seat-bar {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
}

.seat {
  background: var(--bg-card);
  padding: 8px;
  border-radius: var(--radius);
}

.seat-head {
  margin-bottom: 4px;
}

.status { margin-top: 4px; }
.badge {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 999px;
  background: #444;
  font-size: 12px;
}

</style>
