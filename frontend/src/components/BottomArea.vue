<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'
import PlayedCardItem from './PlayedCardItem.vue'

type Card = { id: number; suit: string; rank: string }

const game = useGameStore()
const v = computed(() => game.view)

/** 常量：升级规则固定 8 张底 */
const BOTTOM_SIZE = 8

const phase = computed(() => v.value?.phase)
const mySeat = computed(() => v.value?.mySeat ?? -1)
const ownerSeat = computed(() => v.value?.bottomOwnerSeat ?? -1)

/** 扣底阶段：仅坐家可见 */
const myBottom = computed<Card[]>(() =>
    (v.value?.myBottom ?? []) as Card[]
)

const canSeeBottom = computed(() =>
    phase.value === 'bottom' &&
    mySeat.value >= 0 &&
    mySeat.value === ownerSeat.value
)

/** 末墩结算：底牌公开（全员可见） */
const bottomReveal = computed<Card[]>(() =>
    (v.value?.bottomReveal ?? []) as Card[]
)

const showBottomReveal = computed(() =>
    v.value?.bottomRevealed === true &&
    bottomReveal.value.length > 0
)
const showBottomArea = computed(() =>
    phase.value === 'bottom' || v.value?.bottomRevealed === true
)
/** 抠底结算信息 */
const bottomPoints = computed(() => v.value?.bottomPoints ?? 0)
const bottomMul = computed(() => v.value?.bottomMul ?? 1)
const bottomAward = computed(() => v.value?.bottomAward ?? 0)
</script>

<template>
  <div v-if="showBottomArea" class="panel bottom-area">
    <h4>底牌（仅供展示）</h4>

    <!-- 扣底阶段：仅坐家可见 -->
    <div v-if="canSeeBottom" class="cards">
      <PlayedCardItem
          v-for="c in myBottom"
          :key="c.id"
          :card="c"
      />
    </div>

    <!-- 末墩结算：全员公开底牌 -->
    <div v-else-if="showBottomReveal" class="cards reveal">
      <PlayedCardItem
          v-for="c in bottomReveal"
          :key="c.id"
          :card="c"
      />
      <div class="meta">
        底牌 {{ bottomPoints }} 分 × {{ bottomMul }} =
        <strong>{{ bottomAward }}</strong>
      </div>
    </div>

    <!-- 3️⃣ 其他阶段：底牌隐藏 -->
    <div v-else class="hint">
      底牌不可见（{{ BOTTOM_SIZE }} 张）
    </div>
  </div>
</template>

<style scoped>
.bottom-area {
  min-height: 70px;
}

.cards {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 6px;
}

/* 末墩公开态，和普通出牌区区分 */
.reveal {
  padding-top: 6px;
  border-top: 1px dashed var(--border-muted);
}

.meta {
  width: 100%;
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

.hint {
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-muted);
}
</style>
