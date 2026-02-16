<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const displayList = computed(() => {
  const arr = game.messages
  return arr.length ? [arr[arr.length - 1]] : []
})
</script>

<template>
  <div class="panel message-panel">
    <div v-if="displayList.length === 0" class="msg notice">
      暂无系统消息
    </div>
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
  height: 50px;
  overflow: hidden;
  padding: 5px;
  display: flex;
  align-items: center;
}


.msg {
  font-size: 18px;
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
.title {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 6px;
  opacity: 0.7;
}


</style>
