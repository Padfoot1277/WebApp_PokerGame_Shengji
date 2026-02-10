<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'
import PlayedCardItem from './PlayedCardItem.vue'

type Card = { id: number; suit: string; rank: string }

const game = useGameStore()
const v = computed(() => game.view)

const phase = computed(() => v.value?.phase)
const mySeat = computed(() => v.value?.mySeat ?? -1)
const owner = computed(() => v.value?.bottomOwnerSeat ?? -1)
const bottomCount = computed(() => v.value?.bottomCount ?? 0) // 若 ViewState 没有这个字段，可删
const myBottom = computed<Card[]>(() => (v.value?.myBottom ?? []) as Card[])

const canSeeBottom = computed(() =>
    phase.value === 'bottom' && mySeat.value >= 0 && mySeat.value === owner.value
)
</script>

<template>
  <div v-if="phase === 'bottom'" class="panel">
    <h4>底牌</h4>

    <div v-if="canSeeBottom" class="cards">
      <PlayedCardItem v-for="c in myBottom" :key="c.id" :card="c" />
    </div>

    <div v-else class="hint">
      底牌不可见（{{ bottomCount || 8 }} 张）
    </div>
  </div>
</template>

<style scoped>
.cards {
  display: flex;
  flex-wrap: wrap;
  margin-top: 6px;
}
.hint {
  margin-top: 6px;
  color: var(--text-muted);
  font-size: 12px;
}
</style>
