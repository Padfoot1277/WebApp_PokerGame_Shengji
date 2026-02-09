<script setup lang="ts">
import { computed } from 'vue'

type Card = {
  id: number
  suit: string
  rank: string
}

const props = defineProps<{
  card: Card
  selected: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle', id: number): void
}>()

const label = computed(() => {
  const suitMap: Record<string, string> = {
    S: '♠️',
    H: '♥️',
    C: '♣️',
    D: '♦️',
    SJ: '🃏',
    BJ: '👑',
  }
  return `${suitMap[props.card.suit] ?? ''}${props.card.rank}`
})
</script>

<template>
  <button
      class="card"
      :class="{ selected }"
      @click="emit('toggle', card.id)"
  >
    {{ label }}
  </button>
</template>

<style scoped>
.card {
  min-width: 44px;
  height: 60px;
  margin: 2px;
  border-radius: 6px;
  background: #555;
  color: white;
  border: none;
}

.card.selected {
  outline: 2px solid #4da3ff;
  transform: translateY(-4px);
}
</style>
