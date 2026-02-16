<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const props = defineProps<{
  selectedIds: number[]
}>()

const emit = defineEmits<{
  (e: 'clear-selection'): void
}>()

const game = useGameStore()
const v = computed(() => game.view)

const mySeat = computed(() => v.value?.mySeat ?? -1)
const phase = computed(() => v.value?.phase)
const trick = computed(() => v.value?.trick)

function clear() {
  emit('clear-selection')
}

/* -----------------------
 * “高亮（可操作）”条件：只看权限
 * ---------------------- */

const canPlayCardsNow = computed(() =>
    !!phase.value &&
    ['play_trick', 'follow_trick'].includes(phase.value) &&
    mySeat.value >= 0 &&
    mySeat.value === trick.value?.turnSeat
)

const canCallTrumpNow = computed(() =>
    phase.value === 'call_trump' &&
    mySeat.value >= 0 &&
    (v.value.callMode === 'race'
        ? v.value.trump.callerSeat === -1
        : mySeat.value === v.value.callTurnSeat)
)

const canPutBottomNow = computed(() =>
    phase.value === 'bottom' &&
    mySeat.value >= 0 &&
    mySeat.value === v.value.bottomOwnerSeat
)

const canTrumpFightNow = computed(() =>
    phase.value === 'trump_fight' &&
    mySeat.value >= 0 &&
    mySeat.value !== v.value.bottomOwnerSeat
)

const canChangeTrumpNow = computed(() =>
    canTrumpFightNow.value &&
    v.value.trump.locked === false // 锁主不允许改主
)

const canAttackTrumpNow = computed(() =>
    canTrumpFightNow.value
)

const canCallPassNow = computed(() => {
  if (mySeat.value < 0) return false
  if (phase.value === 'call_trump') return !v.value.callPassedSeats[mySeat.value]
  if (phase.value === 'trump_fight') {
    if (mySeat.value === v.value.bottomOwnerSeat) return false
    return !v.value.fightPassedSeats[mySeat.value]
  }
  return false
})

const canNextRoundNow = computed(() => {
  if (mySeat.value < 0) return false
  if (phase.value !== 'round_settle') return false
  return mySeat.value === v.value?.nextStarterSeat
})


function nextRound() {
  if (!canNextRoundNow.value) return
  game.sendEvent('game.start_next_round', {})
}

/* -----------------------
 * 点击提交：做“最小形状校验”，不做规则校验
 * ---------------------- */

function playCards() {
  if (!canPlayCardsNow.value) return
  if (props.selectedIds.length === 0) {
    game.pushMessage('error', '请先选择要出的牌')
    return
  }
  game.sendEvent('game.play_cards', { cardIds: props.selectedIds })
}

type Card = { id: number; suit: string; rank: string }

function getCardsByIds(ids: number[]): Card[] {
  const hand2D: Card[][] = (v.value?.myHand ?? []) as Card[][]
  const flat = hand2D.flat()

  const map = new Map<number, Card>()
  for (const c of flat) {
    if (c) map.set(c.id, c)
  }

  return ids
      .map((id) => map.get(id))
      .filter((c): c is Card => Boolean(c))
}

function isJoker(card: Card): boolean {
  return card.suit === 'SJ' || card.suit === 'BJ' || card.suit === '小王' || card.suit === '大王'
}

function splitJokers(ids: number[]) {
  console.log(ids)
  const cards = getCardsByIds(ids)
  console.log(cards)
  const jokers: Card[] = []
  const others: Card[] = []
  for (const c of cards) {
    if (isJoker(c)) jokers.push(c)
    else others.push(c)
  }
  return { jokers, others }
}

function callTrump() {
  if (!canCallTrumpNow.value) return

  // 允许：1 joker + (1 or 2) level
  if (!v.value?.myHand || v.value.myHand.length === 0) {
    game.pushMessage('error', '手牌尚未同步，无法操作')
    return
  }
  const { jokers, others } = splitJokers(props.selectedIds)

  if (jokers.length !== 1) {
    game.pushMessage('error', '定主：请选择且仅选择 1 张王')
    return
  }
  if (others.length !== 1 && others.length !== 2) {
    game.pushMessage('error', '定主：还需选择 1~2 张级牌')
    return
  }

  game.sendEvent('game.call_trump', {
    jokerId: jokers[0].id,
    levelIds: others.map((c) => c.id),
  })
}

function changeTrump() {
  if (!canChangeTrumpNow.value) return

  // 形状：1 joker + 2 level
  if (!v.value?.myHand || v.value.myHand.length === 0) {
    game.pushMessage('error', '手牌尚未同步，无法操作')
    return
  }
  const { jokers, others } = splitJokers(props.selectedIds)

  if (jokers.length !== 1) {
    game.pushMessage('error', '改主：请选择且仅选择 1 张王')
    return
  }
  if (others.length !== 2) {
    game.pushMessage('error', '改主：还需选择 2 张级牌')
    return
  }

  game.sendEvent('game.change_trump', {
    jokerId: jokers[0].id,
    levelIds: others.map((c) => c.id),
  })
}

function attackTrump() {
  if (!canAttackTrumpNow.value) return

  // 形状：2 jokers（不要求顺序；同类校验交给后端）
  if (!v.value?.myHand || v.value.myHand.length === 0) {
    game.pushMessage('error', '手牌尚未同步，无法操作')
    return
  }
  const { jokers, others } = splitJokers(props.selectedIds)

  if (others.length !== 0) {
    game.pushMessage('error', '攻主：只能选择王牌（2 张）')
    return
  }
  if (jokers.length !== 2) {
    game.pushMessage('error', '攻主：请选择正好 2 张王')
    return
  }

  game.sendEvent('game.attack_trump', {
    jokerIds: jokers.map((c) => c.id),
  })
}


function putBottom() {
  if (!canPutBottomNow.value) return
  if (props.selectedIds.length !== 8) {
    game.pushMessage('error', '扣底必须选择正好 8 张牌')
    return
  }
  game.sendEvent('game.put_bottom', { discardIds: props.selectedIds })
}

function callPass() {
  if (!canCallPassNow.value) return
  game.sendEvent('game.call_pass', {})
}
</script>

<template>
  <div class="panel action-bar">
    <!-- 出牌阶段 -->
    <button @click="playCards" :disabled="!canPlayCardsNow">出牌</button>

    <!-- 定主阶段 -->
    <button @click="callTrump" :disabled="!canCallTrumpNow">定主</button>

    <!-- 扣底阶段 -->
    <button @click="putBottom" :disabled="!canPutBottomNow">扣底</button>

    <!-- 改主/攻主阶段 -->
    <button @click="changeTrump" :disabled="!canChangeTrumpNow">改主</button>
    <button @click="attackTrump" :disabled="!canAttackTrumpNow">攻主</button>

    <!-- 通用 -->
    <button @click="callPass" :disabled="!canCallPassNow">跳过</button>
    <button @click="clear">清空选择</button>

    <!-- 下一小局 -->
    <button @click="nextRound"  :disabled="!canNextRoundNow">下一小局</button>

  </div>
</template>

<style scoped>
.action-bar {
  margin-top: 8px;
}

button {
  background: #3498db;
  margin-right: 6px;
  margin-bottom: 6px;
  min-width: 60px;
  min-height: 44px;
  border: 1px solid transparent;
}

button:not(:disabled) {
  border-color: #4da3ff; /* 可操作即高亮 */
}

button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>
