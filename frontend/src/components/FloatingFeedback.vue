<script setup lang="ts">
import { ref, watch, onUnmounted } from "vue";
import { useGameStore } from "../store/game";
import { floatingFeedback } from "../utils/play-feedback";
const game = useGameStore();
const lines = ref<string[]>([]);
const errorTone = ref(false);
let timer: ReturnType<typeof setTimeout> | undefined;
let baseline = true;
watch(
  () => game.wsStatus,
  () => {
    baseline = true;
    lines.value = [];
    clearTimeout(timer);
  },
);
watch(
  () => game.view,
  (next, before) => {
    if (!next || baseline) {
      baseline = false;
      return;
    }
    const feedback = floatingFeedback(before ?? null, next);
    if (!feedback.length) return;
    if (errorTone.value && lines.value.length) return;
    errorTone.value = false;
    lines.value = feedback;
    clearTimeout(timer);
    timer = setTimeout(() => {
      lines.value = [];
    }, 2200);
  },
  { immediate: true },
);
watch(
  () => game.messages[game.messages.length - 1],
  (message) => {
    if (message?.level !== "error") return;
    errorTone.value = true;
    lines.value = [message.text.replace(/ \([A-Z_]+\)$/, "")];
    clearTimeout(timer);
    timer = setTimeout(() => {
      lines.value = [];
      errorTone.value = false;
    }, 3200);
  },
);
onUnmounted(() => clearTimeout(timer));
</script>
<template>
  <div class="floating-feedback" role="status" aria-live="polite">
    <div
      v-for="line in lines"
      :key="line"
      class="toast"
      :class="{ error: errorTone }"
    >
      {{ line }}
    </div>
  </div>
</template>
<style scoped>
.floating-feedback {
  position: fixed;
  top: 18vh;
  left: 50%;
  transform: translateX(-50%);
  z-index: 30;
  pointer-events: none;
  display: grid;
  gap: 6px;
  width: max-content;
  max-width: 90vw;
}
.toast {
  padding: 10px 18px;
  background: #403569;
  color: #fff;
  border: 1px solid #baabef;
  border-radius: 14px;
  box-shadow: 0 5px 18px #34235b30;
  font-weight: 600;
  text-align: center;
  animation: appear 0.18s ease-out;
}
.toast.error {
  background: #fff0ed;
  color: #b14539;
  border-color: #efc8c1;
}
@keyframes appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
@media (prefers-reduced-motion: reduce) {
  .toast {
    animation: none;
  }
}
</style>
