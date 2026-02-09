<script setup lang="ts">
import { computed } from 'vue'

type Card = {
  id: number
  suit: string
  rank: string
}

const props = defineProps<{
  card: Card
}>()

const label = computed(() => {
  const suitMap: Record<string, string> = {
    S: '♠️',
    H: '♥️',
    C: '♣️',
    D: '♦️',
    SJ: '🃏', // 小王
    BJ: '👑', // 大王（也可用 🃏🃏 区分）
  }
  return `${suitMap[props.card.suit] ?? ''}${props.card.rank}`
})
</script>

<template>
  <button class="played-card" disabled>
    {{ label }}
  </button>
</template>

<style scoped>
.played-card {
  min-width: 44px;
  height: 44px;
  margin: 2px;
  border-radius: 6px;
  background: #444;
  color: #fff;
  border: 1px solid #666;
  opacity: 0.95;
  cursor: default;
}
</style>
