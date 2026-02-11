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
        队伍A (0/2)：<strong>{{ teams[0].levelRank }}</strong> 级，   队伍B (1/3)：<strong>{{ teams[1].levelRank }}</strong> 级
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
