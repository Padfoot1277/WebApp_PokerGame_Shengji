<script setup lang="ts">
import { computed } from 'vue'
import { suitToSymbol } from '../utils/card'

const suitInfo = computed(() => suitToSymbol(props.card.suit))

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

</script>

<template>
  <button
      class="card"
      :class="{ selected }"
      @click="emit('toggle', card.id)"
  >
  <span
      class="suit"
      :class="suitInfo.color"
  >
    {{ suitInfo.symbol }}
  </span>
    <span class="rank">
    {{ card.rank }}
  </span>
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
  font-size: 20px;
}

.suit.black {
  color: #000;              /* 纯黑 */
}

.suit.red {
  color: #e53935;           /* 稍亮的红，深色背景不刺眼 */
}

.suit.joker {
  color: #ffd54f;           /* 金色，和👑协调 */
}

.rank {
  margin-left: 2px;
}

.card.selected {
  outline: 2px solid #4da3ff;
  transform: translateY(-4px);
}
</style>
