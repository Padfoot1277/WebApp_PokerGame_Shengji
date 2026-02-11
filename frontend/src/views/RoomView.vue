<script setup lang="ts">
import ConnectionPanel from '../components/ConnectionPanel.vue'
import SeatList from '../components/SeatList.vue'
</script>

<template>
  <div class="view-container">
    <h2>房间界面（Lobby）</h2>

    <ConnectionPanel />

    <SeatList />
  </div>
</template>
