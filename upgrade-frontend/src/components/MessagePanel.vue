<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()
const boxRef = ref<HTMLDivElement | null>(null)

// 最新在上：倒序渲染（新 -> 旧）
const displayList = computed(() => {
  // slice() 防止原地 reverse 影响 store
  return game.messages.slice().reverse()
})

// 每次消息数量变化：滚到顶部（最新位置）
watch(
    () => game.messages.length,
    async () => {
      await nextTick()
      if (boxRef.value) {
        boxRef.value.scrollTop = 0
      }
    }
)
</script>

<template>
  <div class="panel message-panel" ref="boxRef">
    <div
        v-for="m in displayList"
        :key="m.id"
        :class="['msg', m.level]"
    >
      {{ m.text }}
    </div>
  </div>
</template>

<style scoped>
.message-panel {
  max-height: 160px;
  overflow-y: auto;
  padding: 10px;
}

.msg {
  font-size: 12px;
  line-height: 1.4;
  margin-bottom: 6px;
  word-break: break-word;
}

.msg.error {
  color: var(--color-error);
}

.msg.notice {
  color: var(--color-notice);
}
</style>
