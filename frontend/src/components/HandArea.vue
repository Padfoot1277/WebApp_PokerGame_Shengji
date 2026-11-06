<script setup lang="ts">
import { computed, watch, ref } from "vue";
import { useGameStore } from "../store/game";
import CardItem from "./CardItem.vue";
const props = defineProps<{ selectedIds: number[] }>();
const emit = defineEmits<{
  (e: "update:selectedIds", ids: number[]): void;
  (e: "clear"): void;
}>();
const game = useGameStore();
const dragging = ref(false);
const selectOnDrag = ref(true);
const seen = new Set<number>();
let lastCardId: number | null = null;
const hand = computed(() => game.view?.myHand ?? []);
function select(id: number) {
  const ids = new Set(props.selectedIds);
  if (selectOnDrag.value) ids.add(id);
  else ids.delete(id);
  emit("update:selectedIds", [...ids]);
}
function down(e: PointerEvent) {
  if (!e.isPrimary || e.button !== 0) return;
  const button = (e.target as HTMLElement).closest<HTMLElement>(
    "[data-card-id]",
  );
  if (!button) return;
  const id = Number(button.dataset.cardId);
  lastCardId = id;
  dragging.value = true;
  seen.clear();
  seen.add(id);
  selectOnDrag.value = !props.selectedIds.includes(id);
  select(id);
  (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
}
function move(e: PointerEvent) {
  if (!dragging.value) return;
  const button = document
    .elementFromPoint(e.clientX, e.clientY)
    ?.closest<HTMLElement>("[data-card-id]");
  if (!button) return;
  const id = Number(button.dataset.cardId);
  if (!seen.has(id)) {
    seen.add(id);
    select(id);
  }
}
function pair(id: number) {
  const cards = hand.value.flat();
  const card = cards.find((c) => c.id === id);
  if (!card) return;
  const ids = new Set(props.selectedIds);
  for (const c of cards)
    if (c.suit === card.suit && c.rank === card.rank) ids.add(c.id);
  emit("update:selectedIds", [...ids]);
}
watch(
  () =>
    `${game.view?.roomId}:${game.view?.mySeat}:${game.view?.phase}:${hand.value
      .flat()
      .map((c) => c.id)
      .sort((a, b) => a - b)
      .join(",")}`,
  () => {
    emit("update:selectedIds", []);
    emit("clear");
  },
);
</script>
<template>
  <section class="panel hand-area">
    <h3>
      我的手牌
      <small>{{ hand.flat().length }} 张 · 滑动选牌，双击选同牌对子</small>
    </h3>
    <div v-for="(group, index) in hand" :key="index" class="suit-group">
      <small>{{ group[0]?.suit_class }}</small>
      <div
        class="cards-row"
        @pointerdown="down"
        @dblclick="lastCardId !== null && pair(lastCardId)"
        @pointermove="move"
        @pointerup="dragging = false"
        @pointercancel="dragging = false"
      >
        <div
          v-for="card in group"
          :key="card.id"
          class="card-wrap"
          :data-card-id="card.id"
        >
          <CardItem
            :card="card"
            :selected="selectedIds.includes(card.id)"
            @toggle="
              (id) => {
                selectOnDrag = !selectedIds.includes(id);
                select(id);
              }
            "
          /><small v-if="game.view?.usedTrumpIds.includes(card.id)" class="used"
            >已亮主</small
          >
        </div>
      </div>
    </div>
  </section>
</template>
<style scoped>
.cards-row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  touch-action: pan-y;
  user-select: none;
  padding: 4px 0;
}
.suit-group + .suit-group {
  border-top: 1px solid #edf1ed;
}
.card-wrap {
  touch-action: none;
  position: relative;
}
.used {
  position: absolute;
  bottom: -4px;
  left: 4px;
  font-size: 9px;
  background: #f4dfac;
  color: #6b4b19;
  padding: 0 3px;
  border-radius: 3px;
  pointer-events: none;
}
h3 small {
  font-size: 12px;
  color: #65796f;
  font-weight: normal;
}
.hand-area small {
  color: #65796f;
}
</style>
