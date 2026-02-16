<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()
const v = computed(() => game.view)

/** 花色固定顺序：必须和后端一致 */
const SUITS = ['♠️', '♥️', '♣️', '♦️']

const record = computed(() =>
    v.value?.record ?? null
)

/** 是否显示记牌区（进入发牌后即可展示） */
const showBoard = computed(() =>
    v.value?.hideRecord === false && v.value?.phase === 'play_trick' && record.value !== null
)

/** 工具函数：安全取值 */
function getVal(arr: number[] | undefined, idx: number) {
  if (!arr) return 0
  return arr[idx] ?? 0
}
</script>

<template>
  <div v-if="showBoard" class="panel high-board">
    <h4>记牌器</h4>

    <div v-if="record" class="grid">
      <!-- 王 -->
      <div class="joker-row">
        👑 大王 {{ record.bigJoker ?? 0 }}
        <span class="gap"></span>
        🃏 小王 {{ record.smallJoker ?? 0 }}
      </div>

      <!-- Num -->
      <div class="row">
        <div class="rank">总数</div>
        <div
            v-for="(s, i) in SUITS"
            :key="'A'+i"
            class="cell"
        >
          {{ s }} {{ getVal(record.num, i) }}
        </div>
      </div>

      <!-- A -->
      <div class="row">
        <div class="rank">A</div>
        <div
            v-for="(s, i) in SUITS"
            :key="'A'+i"
            class="cell"
        >
          {{ s }} {{ getVal(record.a, i) }}
        </div>
      </div>

      <!-- K -->
      <div class="row">
        <div class="rank">K</div>
        <div
            v-for="(s, i) in SUITS"
            :key="'K'+i"
            class="cell"
        >
          {{ s }} {{ getVal(record.k, i) }}
        </div>
      </div>

      <!-- 10 -->
      <div class="row">
        <div class="rank">10</div>
        <div
            v-for="(s, i) in SUITS"
            :key="'T'+i"
            class="cell"
        >
          {{ s }} {{ getVal(record.ten, i) }}
        </div>
      </div>

    </div>

    <div v-else class="hint">
      暂无记牌数据
    </div>
  </div>
</template>

<style scoped>
.high-board {
  min-height: 96px;
}

.grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 6px;
}

.row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.rank {
  width: 40px;
  font-weight: 600;
  text-align: center;
}

.cell {
  min-width: 60px;
  font-size: 20px;
  color: var(--text-primary);
}

.joker-row {
  border-top: 1px dashed var(--border-muted);
  font-size: 20px;
}

.gap {
  display: inline-block;
  width: 12px;
}

.hint {
  margin-top: 6px;
  font-size: 15px;
  color: var(--text-muted);
}
</style>
