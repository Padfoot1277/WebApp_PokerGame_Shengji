<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useGameStore } from "./store/game";
import RoomView from "./views/RoomView.vue";
import GameView from "./views/GameView.vue";
import SessionPanel from "./components/SessionPanel.vue";
import MessageLatestPanel from "./components/MessageLatestPanel.vue";
const game = useGameStore();
const inGame = computed(() => game.view && game.view.phase !== "lobby");
onMounted(() => {
  const url = sessionStorage.getItem("shengji-url");
  if (url) game.connect(url);
});
</script>
<template>
  <div class="app-root">
    <SessionPanel /><GameView v-if="inGame" /><RoomView
      v-else
    /><MessageLatestPanel v-if="!inGame" />
    <details class="panel rules">
      <summary>游戏规则与操作帮助</summary>
      <p>
        四人固定搭档，你和对家一队，上家和下家一队。每人 25 张，底牌 8
        张。5、10、K 为分牌，只有打家累计得分。
      </p>
      <p>
        定主：同色王加本队级牌；一对级牌可锁主。攻改按座位依次进行，跳过后不可反悔。每张亮主牌本局只能使用一次。
      </p>
      <p>
        跟牌必须尽可能满足同花色及结构。甩牌可被其余三人任意一人阻止，失败后强制出可被压制的最低牌力结构。
      </p>
      <p>
        末墩抠底倍率：拖拉机 ×4、对子 ×2、单张
        ×1，组合取最高倍率。无人定主或打家攻主不抠底，坐家攻主允许打家抠底。
      </p>
      <p>
        打家得分：0 分坐家升 3 级；1–39 升 2 级；40–79 升 1 级；80–119
        换坐；120–159 打家升 1 级；160–199 升 2 级；200 及以上升 3 级。先升到 A
        获胜。
      </p>
      <p>
        同队两人同意弃局后，按已结算得分结束本局；不计当前未完成墩、不抠底。第二人确认前可以撤回申请。
      </p>
    </details>
  </div>
</template>
<style>
.rules {
  max-width: 1100px;
  margin: 16px auto;
  font-size: 14px;
  line-height: 1.8;
}
.rules summary {
  cursor: pointer;
  font-weight: 600;
}
</style>
