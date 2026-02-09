<script setup lang="ts">
import type { PropType } from 'vue'
import PlayedCardItem from './PlayedCardItem.vue'

type Card = { id: number; suit: string; rank: string }

type Block = {
  type: string
  suitClass: string
  ranks: number
  tractor_len: number
  cards: Card[]
}

type PlayedMove = {
  seat: number
  suitClass: string
  followMove: {
    blocks: Block[][]
    cards: Card[]
  }
  info?: string
}

const props = defineProps({
  move: {
    type: Object as PropType<PlayedMove | null>,
    default: null,
  },
  isLeader: { type: Boolean, default: false },
  isTurn: { type: Boolean, default: false },
})
</script>

<template>
  <div class="trick">
    <div v-if="!move" class="empty">未出牌</div>

    <div v-else>
      <!-- 牌面展示（只读按钮） -->
      <div class="cards">
        <PlayedCardItem
            v-for="c in move.followMove.cards"
            :key="c.id"
            :card="c"
        />
      </div>

      <!-- 可选：结构信息（辅助理解：单/对/拖拉机） -->
      <div class="blocks">
        <span
            v-for="(group, gi) in move.followMove.blocks"
            :key="gi"
            class="block-group"
        >
          <span
              v-for="(blk, bi) in group"
              :key="bi"
              class="block-chip"
          >
            {{ blk.type }}×{{ blk.cards.length }}
          </span>
        </span>
      </div>

      <div class="tags">
        <span v-if="props.isLeader" class="tag">先手</span>
        <span v-if="props.isTurn" class="tag turn">当前轮到</span>
        <span v-if="move.suitClass" class="tag muted">牌域：{{ move.suitClass }}</span>
        <span v-if="move.info" class="tag muted">{{ move.info }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.trick {
  margin-top: 8px;
  font-size: 12px;
}

.empty {
  color: var(--text-muted);
}

.cards {
  display: flex;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.blocks {
  margin-bottom: 4px;
}

.block-chip {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 999px;
  background: #555;
  margin-right: 4px;
  margin-bottom: 4px;
}

.tags .tag {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 999px;
  background: #3a3a3a;
  margin-right: 4px;
  margin-bottom: 4px;
}

.tags .turn {
  border: 1px solid #4da3ff;
}

.tags .muted {
  color: var(--text-muted);
}
</style>
