<script setup lang="ts">
import { computed, watch } from "vue";
import { useGameStore } from "../store/game";
import { unavailableCard } from "../utils/play-feedback";
import CardItem from "./CardItem.vue";
const props = defineProps<{ selectedIds: number[] }>();
const emit = defineEmits<{
  (e: "update:selectedIds", ids: number[]): void;
  (e: "clear"): void;
}>();
const game = useGameStore();
let gesture: { id: number; x: number; y: number; cancelled: boolean } | null =
  null;
let lastCardId: number | null = null;
const hand = computed(() => game.view?.myHand ?? []);
function toggle(id: number) {
  if (game.reviewPrevious || game.pending) return;
  if (
    unavailableCard(
      game.view,
      hand.value.flat().find((c) => c.id === id)!,
      props.selectedIds,
    )
  )
    return;
  const ids = new Set(props.selectedIds);
  if (ids.has(id)) ids.delete(id);
  else ids.add(id);
  emit("update:selectedIds", [...ids]);
}
function down(e: PointerEvent) {
  if (!e.isPrimary || e.button !== 0) return;
  const element = (e.target as HTMLElement).closest<HTMLElement>(
    "[data-card-id]",
  );
  if (!element) return;
  lastCardId = Number(element.dataset.cardId);
  gesture = { id: lastCardId, x: e.clientX, y: e.clientY, cancelled: false };
}
function move(e: PointerEvent) {
  if (gesture && Math.hypot(e.clientX - gesture.x, e.clientY - gesture.y) > 8)
    gesture.cancelled = true;
}
function up(e: PointerEvent) {
  const g = gesture;
  gesture = null;
  if (!g || g.cancelled || Math.hypot(e.clientX - g.x, e.clientY - g.y) > 8)
    return;
  const element = (e.target as HTMLElement).closest<HTMLElement>(
    "[data-card-id]",
  );
  if (Number(element?.dataset.cardId) === g.id) toggle(g.id);
}
function pair(id: number) {
  if (game.reviewPrevious || game.pending) return;
  const cards = hand.value.flat();
  const card = cards.find((c) => c.id === id);
  if (!card || unavailableCard(game.view, card, props.selectedIds)) return;
  const ids = new Set(props.selectedIds);
  for (const c of cards)
    if (
      c.suit === card.suit &&
      c.rank === card.rank &&
      !unavailableCard(game.view, c, props.selectedIds)
    )
      ids.add(c.id);
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
  <section
    class="panel hand-area"
    :class="{ 'read-only': game.reviewPrevious || game.pending }"
    :aria-disabled="game.reviewPrevious || game.pending"
  >
    <h3>
      我的手牌
      <small>{{ hand.flat().length }} 张 · 点击选牌，双击选同牌对子</small>
    </h3>
    <p v-if="game.reviewPrevious" class="read-only-hint">
      回放中，回到当前牌桌后可继续选牌。
    </p>
    <div v-for="(group, index) in hand" :key="index" class="suit-group">
      <small>{{ group[0]?.suit_class }}</small>
      <div
        class="cards-row"
        @pointerdown="down"
        @dblclick="lastCardId !== null && pair(lastCardId)"
        @pointermove="move"
        @pointerup="up"
        @pointercancel="gesture = null"
        @pointerleave="gesture = null"
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
            :unavailable="unavailableCard(game.view, card, selectedIds)"
            @toggle="toggle"
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
  touch-action: pan-y;
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
.read-only .cards-row {
  opacity: 0.65;
}
.read-only-hint {
  color: #315568;
  font-size: 13px;
  margin: 8px 0;
}
</style>
