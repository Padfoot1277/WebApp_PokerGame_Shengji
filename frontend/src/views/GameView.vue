<script setup lang="ts">
import { ref } from 'vue'
import GameStatusBar from '../components/GameStatusBar.vue'
import RoundStatusBar from '../components/RoundStatusBar.vue'
import SeatBar from '../components/SeatBar.vue'
import ActionBar from '../components/ActionBar.vue'
import HandArea from '../components/HandArea.vue'
import MessagePanel from '../components/MessagePanel.vue'
import BottomArea from '../components/BottomArea.vue'
import Record from "../components/Record.vue";

const selectedIds = ref<number[]>([])

function clearSelection() {
  selectedIds.value = []
}
</script>

<template>
  <div class="view-container">
    <GameStatusBar />
    <RoundStatusBar />
    <SeatBar />

    <HandArea
        v-model:selected-ids="selectedIds"
        @clear="clearSelection"
    />
    <ActionBar
        :selected-ids="selectedIds"
        @clear-selection="clearSelection"
    />
    <BottomArea />
    <MessagePanel />
    <Record />
  </div>
</template>
