<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const mySeat = computed(() => game.view?.mySeat ?? -1)
const myReady = computed(() =>
    mySeat.value >= 0 ? game.view.seats[mySeat.value].ready : false
)

function ready() {
  game.sendEvent('room.ready', {})
}

function unready() {
  game.sendEvent('room.unready', {})
}

function startGame() {
  game.sendEvent('game.start', {})
}
</script>

<template>
  <div class="panel">
    <h3>准备</h3>

    <button
        v-if="mySeat >= 0 && !myReady"
        @click="ready"
    >
      准备
    </button>

    <button
        v-if="mySeat >= 0 && myReady"
        @click="unready"
    >
      取消准备
    </button>

<!--    <button @click="startGame">-->
<!--      开始游戏-->
<!--    </button>-->
  </div>
</template>

<style scoped>
.panel {
  background: var(--bg-panel);
  padding: 12px;
  border-radius: var(--radius);
}
</style>
