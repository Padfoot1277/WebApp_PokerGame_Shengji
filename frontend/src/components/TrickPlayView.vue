<script setup lang="ts">
import type { PropType } from "vue";
import PlayedCardItem from "./PlayedCardItem.vue";

type Card = { id: number; suit: string; rank: string };

type Block = {
  type: string;
  suitClass: string;
  ranks: number;
  tractor_len: number;
  cards: Card[];
};

type PlayedMove = {
  seat: number;
  suitClass: string;
  followMove: {
    blocks: Block[][] | null;
    cards: Card[];
  };
  info?: string;
};

const props = defineProps({
  move: {
    type: Object as PropType<PlayedMove | null>,
    default: null,
  },
  isLeader: { type: Boolean, default: false },
  isTurn: { type: Boolean, default: false },
});
</script>

<template>
  <div class="trick-mini">
    <div v-if="!move" class="empty">未出牌</div>

    <template v-else>
      <div class="cards">
        <PlayedCardItem
          v-for="c in move.followMove.cards"
          :key="c.id"
          :card="c"
        />
      </div>

      <p
        v-if="move.info && !['垫牌', '杂牌'].includes(move.info)"
        class="meta"
        :class="{ 'throw-blocked': move.info.startsWith('甩牌被阻止') }"
      >
        {{ move.info }}
      </p>
    </template>
  </div>
</template>

<style scoped>
/* 右上角浮层的“卡片” */
.trick-mini {
  width: 100%; /* 你可以改 180/200/240 */
  max-width: 100%;
  background: transparent;
  border-radius: 10px;
  padding: 2px;
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
  font-size: 12px;
}

.meta.throw-blocked {
  color: #c53939;
}

.meta-item {
  color: var(--text-muted);
}
</style>
