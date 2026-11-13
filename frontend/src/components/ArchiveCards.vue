<script setup lang="ts">
import type { Card } from "../types/view";
import { suitToSymbol } from "../utils/card";
defineProps<{ cards?: (Card | null)[] | null }>();
</script>
<template>
  <span class="cards"
    ><template v-for="card in cards" :key="card?.id"
      ><span v-if="card" class="pip" :class="suitToSymbol(card.suit).color"
        >{{ suitToSymbol(card.suit).symbol }}{{ card.rank }}</span
      ></template
    ><span v-if="!cards?.length">—</span></span
  >
</template>
<style scoped>
.cards {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}
.pip {
  display: inline-block;
  background: #fffef8;
  border: 1px solid #cec9bb;
  border-radius: 4px;
  padding: 2px 5px;
  font:
    600 14px/1.5 Georgia,
    serif;
  white-space: nowrap;
  color: #223b34;
}
.red {
  color: #ab433d;
}
.joker {
  color: #7e562a;
}
</style>
