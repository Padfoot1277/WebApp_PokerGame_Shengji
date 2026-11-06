<script setup lang="ts">
import { ref } from "vue";
import { useGameStore } from "../store/game";
const game = useGameStore();
const uid = ref(
  sessionStorage.getItem("shengji-nickname") ??
    localStorage.getItem("nickname") ??
    "",
);
const roomId = ref(
  new URLSearchParams(location.search).get("room") ??
    localStorage.getItem("room") ??
    "",
);
const wsBase = ref(
  import.meta.env.VITE_WS_URL ||
    `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}/ws`,
);
function join(create = false) {
  if (create)
    roomId.value = Array.from(crypto.getRandomValues(new Uint8Array(4)), (n) =>
      (n % 36).toString(36),
    )
      .join("")
      .toUpperCase();
  if (!roomId.value.trim()) {
    game.pushMessage("error", "请输入房间码，或创建新房间");
    return;
  }
  try {
    const url = new URL(wsBase.value);
    if (!["ws:", "wss:"].includes(url.protocol)) throw Error();
    url.searchParams.set("room", roomId.value.trim());
    url.searchParams.set("name", uid.value.trim());
    localStorage.setItem("nickname", uid.value.trim());
    localStorage.setItem("room", roomId.value.trim());
    game.connect(url.toString());
  } catch {
    game.pushMessage("error", "服务器地址格式有误");
  }
}
function logout() {
  game.disconnect();
  sessionStorage.removeItem("shengji-token");
}
</script>
<template>
  <section v-if="!game.view" class="panel connect">
    <h2>升级 <small>四人 · 固定对家 · 双副牌</small></h2>
    <label
      >昵称<input v-model="uid" maxlength="24" placeholder="给自己取个名字"
    /></label>
    <label
      >房间码<input
        v-model="roomId"
        maxlength="32"
        placeholder="输入朋友分享的房间码"
        @keyup.enter="join()"
    /></label>
    <div class="actions">
      <button @click="join()">加入房间</button
      ><button class="secondary" @click="join(true)">创建房间</button>
    </div>
    <details>
      <summary>连接设置</summary>
      <label>服务器地址<input v-model="wsBase" /></label>
      <p>昵称仅用于显示，当前浏览器标签页会自动保存身份。</p>
      <button @click="logout">退出身份</button>
    </details>
  </section>
</template>
<style scoped>
.connect {
  width: 100%;
  max-width: 640px;
  margin: auto;
}
label {
  display: flex;
  gap: 12px;
  align-items: center;
  margin: 16px 0;
}
input {
  flex: 1;
  min-width: 0;
  padding: 12px;
  border: 1px solid #cad9cf;
  border-radius: 8px;
  font: inherit;
}
.actions {
  display: flex;
  gap: 12px;
}
small {
  font-size: 13px;
  color: #65796f;
  font-weight: normal;
}
details {
  margin-top: 20px;
  color: #65796f;
}
summary {
  cursor: pointer;
}
</style>
