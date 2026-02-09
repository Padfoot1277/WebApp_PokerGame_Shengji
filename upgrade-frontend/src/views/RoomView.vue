<script setup lang="ts">
import ConnectionPanel from '../components/ConnectionPanel.vue'
import SeatList from '../components/SeatList.vue'
import ReadyPanel from '../components/ReadyPanel.vue'
</script>

<template>
  <div class="view-container">
    <h2>房间界面（Lobby）</h2>

    <ConnectionPanel />

    <SeatList />

    <ReadyPanel />
  </div>
</template>
