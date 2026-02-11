<script setup lang="ts">
import type { PropType } from 'vue'
import { computed } from 'vue'
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

const suitClassText = computed(() => props.move?.suitClass ?? '')
</script>

<template>
  <div class="trick-mini">
    <div v-if="!move" class="empty">未出牌</div>

    <template v-else>
      <div class="cards">
        <PlayedCardItem v-for="c in move.followMove.cards" :key="c.id" :card="c" />
      </div>

      <div class="meta">
        <span v-if="suitClassText" class="meta-item">🧩 {{ suitClassText }}</span>
        <span v-if="move.info" class="meta-item">ℹ️ {{ move.info }}</span>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* 右上角浮层的“卡片” */
.trick-mini {
  width: 100%;              /* 你可以改 180/200/240 */
  max-width: 60vw;
  background: rgba(30, 30, 30, 0.9);
  border: 1px solid #444;
  border-radius: 10px;
  padding: 6px;
  font-size: 12px;
}


/* 标签行 */
.topline {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-bottom: 6px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border-radius: 999px;
  background: #2e2e2e;
  color: #eee;
  line-height: 1.2;
}
.trick-mini {
  margin-top: 6px;
  font-size: 12px;
}

.empty {
  color: var(--text-muted);
}

.cards {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.meta {
  margin-top: 4px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.meta-item {
  color: var(--text-muted);
}
</style>
