<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'
import TrickPlayView from './TrickPlayView.vue'

const game = useGameStore()
const v = computed(() => game.view)

const seats = computed(() => v.value?.seats ?? [])
const seatOrder = [0, 1, 3, 2]

const orderedSeats = computed(() =>
    seatOrder.map((idx) => ({ idx, s: seats.value[idx] }))
)

const trickToShow = computed(() => {
  const t = v.value?.trick
  if (!t) return null

  // 已结算：展示上一墩
  if (t.resolved && t.lastMoves) {
    return {
      ...t,
      playedMoves: t.lastMoves,
    }
  }

  // 未结算：展示当前墩
  return t
})
const liveTrick = computed(() => v.value?.trick)



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
  // 状态判断应基于“当前真实 trick”（而非 displayTrick），避免上一墩冻结时误导“轮到谁”
  if (view.phase === 'play_trick' || view.phase === 'follow_trick') {
    const pm = view.trick?.playedMoves?.[idx]
    const hasPlayed = !!pm
    if (hasPlayed) return idx === view.trick.leaderSeat ? '已出牌（先手）' : '已出牌'
    if (idx === view.trick.turnSeat) return '出牌中（轮到）'
    return '等待出牌'
  }

  return ''
}
function isActiveSeat(idx: number): boolean {
  const view = v.value
  if (!view) return false

  if (view.phase === 'call_trump') {
    if (view.callMode === 'ordered') return idx === view.callTurnSeat
    // race：没有“轮到谁”，你可选择不高亮，或高亮所有未pass且仍可抢的人
    return view.trump.callerSeat === -1 && !view.callPassedSeats[idx]
  }

  if (view.phase === 'bottom') {
    return idx === view.bottomOwnerSeat
  }

  if (view.phase === 'trump_fight') {
    if (idx === view.bottomOwnerSeat) return false
    return !view.fightPassedSeats[idx]
  }

  if (view.phase === 'play_trick' || view.phase === 'follow_trick') {
    return idx === view.trick?.turnSeat
  }

  return false
}



</script>

<template>
  <div class="seat-bar">
    <div
        v-for="item in orderedSeats"
        :key="item.idx"
        class="seat"
        :class="{
    me: item.idx === v?.mySeat,
    active: isActiveSeat(item.idx),
  }"
    >
      <div class="seat-head">
        <strong>Seat {{ item.idx }}</strong>
        <span v-if="item.idx === v?.mySeat">（我）</span>
      </div>

      <div class="status">
        状态：<span class="badge">{{ seatStatus(item.idx) }}</span>
      </div>

      <div>UID: {{ item.s.uid || '空' }}</div>

      <!-- ✅ 右上角浮层 -->
      <div class="corner-badges">
        <span v-if="item.idx === trickToShow?.leaderSeat" class="badge leader" title="先手">🚩</span>
        <span v-if="item.idx === liveTrick?.turnSeat" class="badge turn" title="轮到">👉</span>
      </div>

      <TrickPlayView
          :move="trickToShow?.playedMoves?.[item.idx] ?? null"
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

.seat.me {
  outline: 2px solid #4da3ff; /* 蓝框 */
}

.seat.active {
  box-shadow: 0 0 0 2px #f5d000 inset; /* 黄框 */
}

.seat {
  position: relative;
  /* 你原来的样式保持 */
}

/* 右上角角标：真正浮动，不占布局 */
/* 右上角角标：真正浮动，不占布局 */
.corner-badges {
  position: absolute;
  top: 6px;
  right: 6px;
  display: flex;
  gap: 8px;
  z-index: 3;
  pointer-events: none; /* 不挡点击 */
}

/* 每一个角标的外观（这里调大） */
.corner-badges .badge {
  width: 30px;
  height: 30px;
  border-radius: 8px;

  display: inline-flex;
  align-items: center;
  justify-content: center;

  font-size: 18px;
  line-height: 1;

  background: rgba(30, 30, 30, 0.8);
  border: 1px solid #444;
}

/* 轮到 / 先手：用边框强调 */
.corner-badges .badge.turn {
  border-color: #f5d000;
}
.corner-badges .badge.leader {
  border-color: pink;
}


</style>
