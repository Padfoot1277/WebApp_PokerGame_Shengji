<script setup lang="ts">
import { ref } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const wsUrl = ref('ws://localhost:8080/ws')   // 按你的后端实际地址改
const roomId = ref('room1')

function connect() {
  if (!wsUrl.value) return
  game.connect(`${wsUrl.value}?room=${roomId.value}`)
}
</script>

<template>
  <div class="panel">
    <h3>连接</h3>

    <div class="row">
      <label>WS 地址</label>
      <input v-model="wsUrl" />
    </div>

    <div class="row">
      <label>房间</label>
      <input v-model="roomId" />
    </div>

    <button @click="connect" :disabled="game.connected">
      {{ game.connected ? '已连接' : '连接' }}
    </button>

    <p v-if="game.uid">UID：{{ game.uid }}</p>
    <p>WS：{{ game.wsStatus }}</p>
  </div>
</template>

<style scoped>
.panel {
  background: var(--bg-panel);
  padding: 12px;
  border-radius: var(--radius);
}

.row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

input {
  flex: 1;
}
</style>
