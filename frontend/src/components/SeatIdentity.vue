<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";

const props = defineProps<{
  name: string;
  tags: { key: string; text: string; label: string; tone?: string }[];
}>();
const viewport = ref<HTMLElement | null>(null);
const content = ref<HTMLElement | null>(null);
const scrolling = ref(false);
const distance = ref(0);
const tagLabel = computed(() => props.tags.map((tag) => tag.label).join("，"));
let observer: ResizeObserver | undefined;

function measure() {
  const width = content.value?.getBoundingClientRect().width ?? 0;
  const available = viewport.value?.clientWidth ?? 0;
  scrolling.value = available > 0 && width > available + 1;
  distance.value = width + 6;
}
onMounted(() => {
  observer = new ResizeObserver(measure);
  if (viewport.value) observer.observe(viewport.value);
  if (content.value) observer.observe(content.value);
  measure();
});
onUnmounted(() => observer?.disconnect());
watch(() => props.tags, measure, { flush: "post", deep: true });
</script>

<template>
  <div class="seat-identity" :class="{ 'has-tags': tags.length }">
    <strong class="nickname" :title="name">{{ name }}</strong>
    <div
      v-show="tags.length"
      ref="viewport"
      class="tag-viewport"
      :class="{ scrolling }"
      role="group"
      :aria-label="tagLabel"
      :title="tagLabel"
      :tabindex="scrolling ? 0 : undefined"
      :style="{
        '--travel': `-${distance}px`,
        '--duration': `${Math.max(6, distance / 24)}s`,
      }"
    >
      <div class="tag-track" aria-hidden="true">
        <div ref="content" class="tag-copy">
          <span
            v-for="tag in tags"
            :key="tag.key"
            class="chip"
            :class="tag.tone"
          >
            {{ tag.text }}
          </span>
        </div>
        <div v-if="scrolling" class="tag-copy duplicate">
          <span
            v-for="tag in tags"
            :key="tag.key"
            class="chip"
            :class="tag.tone"
          >
            {{ tag.text }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.seat-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.nickname {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.has-tags .nickname {
  flex: 0 1 auto;
  max-width: min(55%, calc(100% - 64px));
}
.tag-viewport {
  flex: 1 1 0;
  min-width: 0;
  overflow: hidden;
  color: #466557;
  font-size: 12px;
}
.tag-track,
.tag-copy {
  display: flex;
  align-items: center;
  gap: 6px;
  width: max-content;
}
.tag-track {
  margin-left: auto;
}
.tag-copy {
  flex-shrink: 0;
}
.scrolling .tag-track {
  animation: tag-marquee var(--duration) linear infinite;
}
.scrolling:hover .tag-track,
.scrolling:focus-visible .tag-track {
  animation-play-state: paused;
}
@keyframes tag-marquee {
  to {
    transform: translateX(var(--travel));
  }
}
.chip {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  padding: 3px 7px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.78);
  line-height: 1.3;
  white-space: nowrap;
}
.chip.turn {
  color: #79531b;
}
.chip.biggest {
  color: #6043a0;
}
@media (max-width: 759px) {
  .tag-viewport {
    font-size: 11px;
  }
  .chip {
    padding: 2px 4px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .scrolling {
    overflow-x: auto;
  }
  .scrolling .tag-track {
    animation: none;
  }
  .duplicate {
    display: none;
  }
}
</style>
