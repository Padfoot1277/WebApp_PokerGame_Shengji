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
const hand = computed<Card[][]>(() => game.view?.myHand ?? [])
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

// 手牌总数变化 -> 清空
const totalCount = computed(() =>
    hand.value.reduce((sum, group) => sum + group.length, 0)
)

watch(totalCount, () => clear())
</script>

<template>
  <div class="panel hand-area">
    <h4>手牌</h4>

    <div class="suit-groups">
      <!-- 外层 suitClass -->
      <div
          v-for="(group, i) in hand"
          :key="i"
          class="cards-row"
      >
        <!-- 内层 具体牌 -->
        <CardItem
            v-for="c in group"
            :key="c.id"
            :card="c"
            :selected="selected.has(c.id)"
            @toggle="toggle"
        />
      </div>
    </div>

  </div>
</template>

<style scoped>
.hand-area {
  margin-top: 8px;
}

/* 整体纵向排列 */
.suit-groups {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

/* 每一行横向排列 */
.cards-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
