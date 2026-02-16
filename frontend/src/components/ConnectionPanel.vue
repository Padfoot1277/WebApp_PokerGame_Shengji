<script setup lang="ts">
import { ref } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const wsBase = ref('ws://192.168.1.109:8080/ws')  // 请替换为当前服务器的IP地址，我这里是192.168.1.109，监听的是8080端口
const roomId = ref('room1')

// 允许为空：为空则后端生成 anon-xxx
const uid = ref(localStorage.getItem('uid') ?? '')

function connect() {
  const room = encodeURIComponent(roomId.value.trim() || 'default')
  const u = uid.value.trim()

  // 只要用户填了，就传 uid；为空就不传，走后端默认
  const url =
      u.length > 0
          ? `${wsBase.value}?room=${room}&uid=${encodeURIComponent(u)}`
          : `${wsBase.value}?room=${room}`

  localStorage.setItem('uid', u) // 记住上次输入（可为空）
  game.connect(url)
}
</script>

<template>
  <div class="panel">
    <h3>房间信息</h3>

    <div class="row">
      <label>用户ID</label>
      <input v-model="uid" placeholder="（可空，将根据时间赋值）" />
    </div>

    <div class="row">
      <label>房间ID</label>
      <input v-model="roomId" />
    </div>

    <div class="row">
      <label>WS地址</label>
      <input v-model="wsBase" />
    </div>

    <button @click="connect">
      连接
    </button>

    <div class="hint">
      当前用户ID：{{ game.uid ?? '未连接' }}
    </div>
  </div>
</template>

<style scoped>
.panel { background: var(--bg-panel); padding: 12px; border-radius: var(--radius); }
.row { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
label { width: 110px; color: var(--text-muted); font-size: 12px; }
input { flex: 1; min-height: 36px; padding: 0 8px; }
.hint { margin-top: 8px; font-size: 12px; color: var(--text-muted); }
.panel button {
  background: #4da3ff;
  color: #fff;
}
@media (hover: hover) and (pointer: fine) {
  .panel button:hover {
    filter: brightness(1.1);
  }
}
</style>
