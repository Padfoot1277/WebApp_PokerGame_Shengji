<script setup lang="ts">
import { computed } from 'vue'
import { suitToSymbol } from '../utils/card'

type Card = {
  id: number
  suit: string
  rank: string
}

const props = defineProps<{
  card: Card
}>()

const suitInfo = computed(() => suitToSymbol(props.card.suit))
</script>

<template>
  <!-- 只读：disabled + pointer-events none，防止任何点击反馈 -->
  <button class="played-card" disabled>
    <span class="suit" :class="suitInfo.color">{{ suitInfo.symbol }}</span>
    <span class="rank">{{ card.rank }}</span>
  </button>
</template>

<style scoped>
/* 尽量对齐 HandArea 中 .card 的“牌面按钮”外观 */
.played-card {
  min-width: 44px;
  height: 40px;
  margin: 2px;
  border-radius: 6px;

  background: #eeeeee;          /* 与手牌一致的浅背景 */
  border: 1px solid #5a5a5a;

  font-size: 20px;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;

  /* 只读，不要像“禁用控件”那样发灰 */
  opacity: 1;
  cursor: default;
  pointer-events: none;          /* 防止 hover/active 等交互 */
}

/* suit / rank 默认颜色逻辑：与手牌区一致 */
.suit.black {
  color: #000;                  /* 纯黑 */
}
.suit.red {
  color: #e53935;
}
.suit.joker {
  color: #ffd54f;
}

/* rank 默认用黑色（手牌区你覆盖到了黑色） */
.rank {
  margin-left: 2px;
  color: #000;
}

/* 关键：disabled 状态下浏览器可能会把文字变灰，这里强行保持一致 */
.played-card:disabled {
  -webkit-text-fill-color: currentColor; /* Safari/Chrome 某些情况下需要 */
  color: #000;
}
</style>
