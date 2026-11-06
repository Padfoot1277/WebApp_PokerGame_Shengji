<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
const props = defineProps<{ selectedIds: number[] }>();
const emit = defineEmits<{ (e: "clear-selection"): void }>();
const game = useGameStore();
const v = computed(() => game.view);
const enabled = computed(
  () =>
    game.wsStatus === "open" &&
    !game.pending &&
    !game.reviewPrevious &&
    (v.value?.mySeat ?? -1) >= 0,
);
const canCall = computed(() => {
  const s = v.value;
  return (
    !!s &&
    s.mySeat >= 0 &&
    s.phase === "call_trump" &&
    !s.callPassedSeats[s.mySeat] &&
    (s.callMode === "race" || s.mySeat === s.callTurnSeat)
  );
});
const canFight = computed(
  () =>
    v.value?.phase === "trump_fight" &&
    v.value.mySeat === v.value.fightTurnSeat,
);
const canPlay = computed(
  () =>
    v.value?.phase === "play_trick" &&
    v.value.mySeat === v.value.trick.turnSeat,
);
const canBottom = computed(
  () =>
    v.value?.phase === "bottom" && v.value.mySeat === v.value.bottomOwnerSeat,
);
const feedback = computed(() => game.messages[game.messages.length - 1]);
const needed = computed(() => {
  const s = v.value;
  if (!s) return 0;
  if (canBottom.value) return 8;
  if (canPlay.value)
    return (
      s.trick.playedMoves[s.trick.leaderSeat]?.followMove.cards.length ?? 0
    );
  return 0;
});
const selectionHelp = computed(() =>
  props.selectedIds.length === needed.value
    ? "张数已满足，请确认跟牌结构"
    : props.selectedIds.length < needed.value
      ? `还需选择 ${needed.value - props.selectedIds.length} 张`
      : `多选了 ${props.selectedIds.length - needed.value} 张，请取消部分选择`,
);
function send(type: string) {
  const s = v.value;
  if (!s || !enabled.value) return;
  const cards = (s.myHand ?? [])
    .flat()
    .filter((c) => props.selectedIds.includes(c.id));
  const jokers = cards.filter((c) => c.suit === "SJ" || c.suit === "BJ");
  const levels = cards.filter((c) => c.suit !== "SJ" && c.suit !== "BJ");
  if (type === "game.play_cards") {
    if (!cards.length) {
      game.pushMessage("error", "请先选牌");
      return;
    }
    game.sendEvent(type, { cardIds: props.selectedIds });
    return;
  }
  if (type === "game.put_bottom") {
    if (cards.length !== 8) {
      game.pushMessage("error", "扣底需要选择 8 张牌");
      return;
    }
    game.sendEvent(type, { discardIds: props.selectedIds });
    return;
  }
  if (type === "game.attack_trump") {
    if (cards.some((c) => s.usedTrumpIds.includes(c.id))) {
      game.pushMessage("error", "选牌中有已用于亮主的牌");
      return;
    }
    if (
      jokers.length !== 2 ||
      levels.length ||
      jokers[0]?.suit !== jokers[1]?.suit
    ) {
      game.pushMessage("error", "请选择两张同类王");
      return;
    }
    game.sendEvent(type, { jokerIds: jokers.map((c) => c.id) });
    return;
  }
  if (
    jokers.length !== 1 ||
    !(type === "game.call_trump" ? [1, 2] : [2]).includes(levels.length)
  ) {
    game.pushMessage("error", "请选择一张王与所需级牌");
    return;
  }
  if (cards.some((c) => s.usedTrumpIds.includes(c.id))) {
    game.pushMessage("error", "选牌中有已用于亮主的牌");
    return;
  }
  const rank =
    type === "game.call_trump"
      ? s.teams[s.mySeat % 2]?.levelRank
      : s.trump.levelRank;
  if (levels.some((c) => c.rank !== rank)) {
    game.pushMessage("error", `请选择 ${rank} 级牌`);
    return;
  }
  if (levels.some((c) => c.suit !== levels[0]?.suit)) {
    game.pushMessage("error", "两张级牌须为同一花色");
    return;
  }
  if ((jokers[0]?.suit === "BJ") !== /[♥♦]/.test(levels[0]?.suit ?? "")) {
    game.pushMessage("error", "大王配红色级牌，小王配黑色级牌");
    return;
  }
  game.sendEvent(type, {
    jokerId: jokers[0]!.id,
    levelIds: levels.map((c) => c.id),
  });
}
</script>
<template>
  <section class="panel action-bar">
    <div class="controls">
      <span
        >已选 <strong>{{ selectedIds.length }}</strong> 张</span
      >
      <template v-if="canCall"
        ><button :disabled="!enabled" @click="send('game.call_trump')">
          定主</button
        ><button
          :disabled="!enabled"
          @click="game.sendEvent('game.call_pass', {})"
        >
          跳过
        </button></template
      >
      <button
        v-if="canBottom"
        :disabled="!enabled"
        @click="send('game.put_bottom')"
      >
        扣下 8 张底牌
      </button>
      <template v-if="canFight"
        ><button
          v-if="!v?.trump.locked"
          :disabled="!enabled"
          @click="send('game.change_trump')"
        >
          改主</button
        ><button :disabled="!enabled" @click="send('game.attack_trump')">
          攻主</button
        ><button
          :disabled="!enabled"
          @click="game.sendEvent('game.call_pass', {})"
        >
          跳过
        </button></template
      >
      <button
        v-if="canPlay"
        :disabled="!enabled"
        @click="send('game.play_cards')"
      >
        出牌
      </button>
      <button v-if="selectedIds.length" @click="emit('clear-selection')">
        清空选择
      </button>
      <button
        v-if="v?.phase === 'round_settle' && v.mySeat === v.nextStarterSeat"
        :disabled="!enabled"
        @click="game.sendEvent('game.start_next_round', {})"
      >
        开始下一小局
      </button>
      <small
        v-if="
          !canCall &&
          !canBottom &&
          !canFight &&
          !canPlay &&
          v?.phase !== 'round_settle'
        "
        >等待其他牌友操作</small
      >
      <small v-if="game.pending">正在确认操作…</small>
      <small v-if="game.reviewPrevious">回放中，请先回到当前牌桌</small>
    </div>
    <p v-if="needed" class="selection-help">{{ selectionHelp }}</p>
    <p v-if="feedback" class="feedback" :class="feedback.level" role="status">
      {{ feedback.text.replace(/ \([A-Z_]+\)$/, "") }}
    </p>
  </section>
</template>
<style scoped>
.action-bar {
  position: sticky;
  bottom: 8px;
  z-index: 4;
  box-shadow: 0 6px 30px #193c2420;
}
.controls {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}
.feedback {
  font-size: 13px;
  color: #52735e;
  margin-top: 8px;
  white-space: normal;
  overflow-wrap: anywhere;
}
.feedback.error {
  color: #b14539;
  background: #fff0ed;
  padding: 8px;
  border-radius: 6px;
}
.selection-help {
  font-size: 12px;
  color: #8b661d;
  margin-top: 6px;
}
</style>
