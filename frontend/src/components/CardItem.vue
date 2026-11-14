<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import { suitToSymbol, isTrumpCard } from "../utils/card";

const game = useGameStore();
const suitInfo = computed(() => suitToSymbol(props.card.suit));

type Card = {
  id: number;
  suit: string;
  rank: string;
};

const props = defineProps<{
  card: Card;
  selected: boolean;
  unavailable?: boolean;
}>();

const emit = defineEmits<{
  (e: "toggle", id: number): void;
}>();

function onClick(e: MouseEvent) {
  if (e.detail === 0) emit("toggle", props.card.id);

  // 关键：手动取消 focus
  const target = e.currentTarget as HTMLButtonElement;
  target.blur();
}
</script>

<template>
  <button
    class="card"
    :class="{
      selected,
      unavailable,
      trump: isTrumpCard(card, game.view?.trump),
    }"
    :aria-pressed="selected"
    :aria-label="`${card.suit}${card.rank}`"
    @click="onClick"
  >
    <span class="suit" :class="suitInfo.color">
      {{ suitInfo.symbol }}
    </span>
    <span class="rank">
      {{ card.rank }}
    </span>
  </button>
</template>

<style scoped>
.card {
  min-width: 36px;
  padding: 3px 6px;
  height: 44px;
  margin: 2px;
  border-radius: 6px;

  border: 1px solid #c3cec6;

  background: #ffffff; /* 默认 */
  color: #000;
  display: flex; /* ⭐ 关键 */
  align-items: center; /* 垂直居中 */
  justify-content: center; /* 水平居中 */
  gap: 2px;
  transition:
    background 0.15s ease,
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    transform 0.1s ease;

  appearance: none;
  -webkit-appearance: none;
  outline: none;
  -webkit-tap-highlight-color: transparent;
}

/* 👇 关键：覆盖所有非 selected 状态 */
.card:not(.selected) {
  background: #ffffff;
  color: #000;
}

/* 选中态 */
.card.selected {
  background: #ecf7ef;
  border: 2px solid #379b80;
  color: #000;
  box-shadow: none;
  filter: none;
  transform: translateY(-4px);
}

/* 点击缩放 */
.card:active {
  transform: scale(0.96);
}

/* ===== 彻底禁用移动端 focus 残留 ===== */

@media (hover: none) {
  .card:focus {
    outline: none;
    box-shadow: none;
  }
}

/* ===== 花色 ===== */

.suit.black {
  color: #000;
}

.suit.red {
  color: #e53935;
}

.suit.joker {
  color: #ffd54f;
}

.rank {
  font-size: 20px;
  font-weight: 500;
}

.suit {
  font-size: 14px;
}
.card.unavailable {
  opacity: 0.6;
  cursor: default;
  box-shadow: none;
}
.card.selected.unavailable {
  opacity: 1;
}
.card.trump {
  border: 3px solid var(--trump-border);
  box-shadow: inset 0 0 0 1px var(--trump-border-highlight);
}
.card.selected.trump {
  border-color: #379b80;
  box-shadow: none;
}
</style>
