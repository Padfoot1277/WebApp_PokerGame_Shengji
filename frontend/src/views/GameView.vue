<script setup lang="ts">
import { ref, onMounted } from "vue";
import GameStatusBar from "../components/GameStatusBar.vue";
import RoundStatusBar from "../components/RoundStatusBar.vue";
import SeatBar from "../components/SeatBar.vue";
import ActionBar from "../components/ActionBar.vue";
import HandArea from "../components/HandArea.vue";
import MessagePanel from "../components/MessagePanel.vue";
import BottomArea from "../components/BottomArea.vue";
import Record from "../components/Record.vue";

import TurnGuide from "../components/TurnGuide.vue";
onMounted(() => window.scrollTo({ top: 0 }));
import { useGameStore } from "../store/game";
const game = useGameStore();
const selectedIds = ref<number[]>([]);

function clearSelection() {
  selectedIds.value = [];
}
</script>

<template>
  <div class="view-container">
    <div class="game-summary"><GameStatusBar /><RoundStatusBar /></div>
    <TurnGuide />
    <div class="play-workspace">
      <SeatBar />
      <div class="hand-workspace">
        <HandArea
          v-if="(game.view?.mySeat ?? -1) >= 0"
          v-model:selected-ids="selectedIds"
          @clear="clearSelection"
        />
        <div v-if="(game.view?.mySeat ?? -1) < 0" class="panel">
          观战模式：仅显示公开出牌和积分，不显示玩家手牌。
        </div>
        <BottomArea />
      </div>
    </div>
    <ActionBar
      v-if="(game.view?.mySeat ?? -1) >= 0"
      :selected-ids="selectedIds"
      @clear-selection="clearSelection"
    />
    <details class="panel">
      <summary>记牌器与对局消息</summary>
      <Record /><MessagePanel />
    </details>
  </div>
</template>

<style scoped>
.game-summary {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.game-summary > * {
  flex: 1;
}
.play-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
}
.hand-workspace {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
summary {
  cursor: pointer;
  font-size: 13px;
  color: #65796f;
}
@media (min-width: 760px) {
  .play-workspace {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    align-items: start;
  }
}
</style>
