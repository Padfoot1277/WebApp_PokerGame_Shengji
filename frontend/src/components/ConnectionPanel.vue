<script setup lang="ts">
import { ref } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()

const wsBase = ref('ws://192.168.1.105:8080/ws')  // 请替换为当前服务器的IP地址，我这里是192.168.1.109，监听的是8080端口
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

  <h3>房间信息</h3>
  <div class="panel">
    <div class="row">
      <label>用户昵称</label>
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

  </div>
</template>

<style scoped>
.panel {
  background: var(--bg-panel);
  padding: 16px 18px;
  border-radius: var(--radius);
}

/* 标题层级更清晰 */
.panel h3 {
  margin: 0 0 14px 0;
  font-size: 18px;
  font-weight: 600;
}

/* 每一行对齐更规整 */
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

/* label 对齐与层级 */
label {
  width: 100px;
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 16px;
  text-align: left;
  letter-spacing: 0.5px;
}

/* 输入框统一尺寸 */
input {
  flex: 1;
  height: 38px;
  padding: 0 10px;
  border-radius: 6px;
  font-size: 14px;
  line-height: 38px;
  box-sizing: border-box;
  border-color: #ffffff;
}

/* 连接按钮 */
.panel button {
  margin-top: 6px;
  height: 38px;
  padding: 0 18px;
  font-size: 16px;
  font-weight: 500;
  border-radius: 6px;
  background: #4da3ff;
  color: #fff;
}

/* hover 保留你原来的逻辑 */
@media (hover: hover) and (pointer: fine) {
  .panel button:hover {
    filter: brightness(1.1);
  }
}

/* 去掉你原本的 !important 覆盖式 hover */
button:hover {
  background: rgba(77, 163, 255, 0.7);
}

/* 底部提示信息 */
.hint {
  margin-top: 12px;
  padding-top: 8px;
  font-size: 12px;
  color: var(--text-muted);
  border-top: 1px solid rgba(2, 2, 2, 0.6);
}

</style>
