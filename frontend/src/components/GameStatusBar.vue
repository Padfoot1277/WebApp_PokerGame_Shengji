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
      <div class="round">
        第 <span class="round-num">{{ roundIndex + 1 }}</span> 小局
      </div>

      <div v-if="teams" class="teams">
        <div class="team">
          <span class="team-label">⓪②队</span>
          <span class="level">{{ teams[0].levelRank }}</span>
          <span class="unit">级</span>
        </div>

        <div class="divider"></div>

        <div class="team">
          <span class="team-label">①③队</span>
          <span class="level">{{ teams[1].levelRank }}</span>
          <span class="unit">级</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 24px;   /* 控制两个元素之间的距离 */
}

.panel {
  padding: 8px 16px;
}

/* 小局标题 */
.round {
  font-size: 16px;
  font-weight: 500;
  color: #333;
}

.round-num {
  font-size: 20px;
  font-weight: 700;
  color: #1a73e8;
}

/* 队伍区域 */
.teams {
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 4px 12px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 8px;
}

/* 单个队伍 */
.team {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.team-label {
  font-size: 15px;
  color: #666;
}

.level {
  font-size: 18px;
  font-weight: 700;
  color: #222;
}

.unit {
  font-size: 15px;
  color: #888;
}

/* 中间分隔线 */
.divider {
  width: 1px;
  height: 18px;
  background: rgba(0, 0, 0, 0.15);
}

</style>
