<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useGameStore } from '../store/game'
import CardItem from './CardItem.vue'

type Card = {
  id: number
  suit: string
  rank: string
}

const props = defineProps<{
  selectedIds: number[]
}>()

const emit = defineEmits<{
  (e: 'update:selectedIds', ids: number[]): void
  (e: 'clear'): void
}>()

const game = useGameStore()
const hand = computed<Card[]>(() => game.view?.myHand ?? [])
const phase = computed(() => game.view?.phase)

const selected = ref<Set<number>>(new Set(props.selectedIds))

watch(
    () => props.selectedIds,
    (ids) => {
      selected.value = new Set(ids)
    }
)

function emitSelection() {
  emit('update:selectedIds', Array.from(selected.value))
}

function toggle(id: number) {
  if (selected.value.has(id)) selected.value.delete(id)
  else selected.value.add(id)
  emitSelection()
}

function clear() {
  if (selected.value.size === 0) return
  selected.value.clear()
  emitSelection()
  emit('clear')
}

// phase 变化 -> 清空
watch(phase, () => clear())

// 手牌数量变化（通常是成功出牌/扣底后）-> 清空
watch(
    () => hand.value.length,
    () => clear()
)
</script>

<template>
  <div class="panel hand-area">
    <h4>手牌</h4>
    <div class="cards">
      <CardItem
          v-for="c in hand"
          :key="c.id"
          :card="c"
          :selected="selected.has(c.id)"
          @toggle="toggle"
      />
    </div>
  </div>
</template>

<style scoped>
.hand-area {
  margin-top: 8px;
}

.hand-area .card {
  color: #000;
  font-size: 20px;      /* ← 推荐 15px 或 16px */
}

.hand-area button {
  background: #eeeeee;        /* 比普通按钮亮 */
  border: 1px solid #5a5a5a;
}

.hand-area button:hover:not(:disabled) {
  background: #e2e2e2; /* 轻微变暗即可 */
}

.hand-area button.selected {
  background: #6b5cff;        /* 已选中的牌，更亮 */
  border-color: #8a80ff;
  color: #fff;
}

.hand-area button:disabled {
  opacity: 0.35;
}

.cards {
  display: flex;
  flex-wrap: wrap;
}
</style>
