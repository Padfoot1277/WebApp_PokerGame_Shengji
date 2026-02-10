<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const teams = computed(() => game.view?.teams)
const roundIndex = computed(() => game.view?.roundIndex ?? 0)
</script>

<template>
  <div class="panel">
    <div class="row">
      <div>第 {{ roundIndex + 1 }} 小局</div>
      <div v-if="teams">
        队伍 A 级牌：{{ teams[0].levelRank }} ｜ 队伍 B 级牌：{{ teams[1].levelRank }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  justify-content: space-between;
}
</style>
