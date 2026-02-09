<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from './store/game'

import RoomView from './views/RoomView.vue'
import GameView from './views/GameView.vue'

const game = useGameStore()

// 临时规则：
// 之后会改成根据 view.phase 判断
const currentView = computed(() => {
  if (!game.view) return 'room'
  return game.view.phase === 'lobby' ? 'room' : 'game'
})
</script>

<template>
  <div class="app-root">
    <RoomView v-if="currentView === 'room'" />
    <GameView v-else />
  </div>
</template>
