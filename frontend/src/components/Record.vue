<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";

const game = useGameStore();
const v = computed(() => game.view);

const SUITS = [
  { key: "spade", label: "♠️" },
  { key: "heart", label: "♥️" },
  { key: "club", label: "♣️" },
  { key: "diamond", label: "♦️" },
] as const;

const record = computed(() => v.value?.record ?? null);

const showBoard = computed(
  () =>
    v.value?.hideRecord === false &&
    v.value?.phase === "play_trick" &&
    record.value !== null,
);
</script>

<template>
  <div v-if="showBoard" class="panel high-board">
    <h4>记牌器</h4>

    <div v-if="record">
      <!-- 王 -->
      <div class="joker-row">
        👑 大王 {{ record.bigJoker ?? 0 }}
        <span class="gap"></span>
        🎩 小王 {{ record.smallJoker ?? 0 }}
      </div>

      <!-- 表格 -->
      <table class="record-table">
        <colgroup>
          <col style="width: 20%" />
          <col style="width: 20%" />
          <col style="width: 20%" />
          <col style="width: 20%" />
          <col style="width: 20%" />
        </colgroup>
        <thead>
          <tr>
            <th>花色</th>
            <th>总数</th>
            <th>A</th>
            <th>K</th>
            <th>10</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in SUITS" :key="s.key">
            <td class="suit">{{ s.label }}</td>
            <td>{{ record[s.key].num }}</td>
            <td>{{ record[s.key].a }}</td>
            <td>{{ record[s.key].k }}</td>
            <td>{{ record[s.key].ten }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-else class="hint">暂无记牌数据</div>
  </div>
</template>
<style scoped>
.high-board {
  min-height: 120px;
  padding: 8px 0;
}

/* 标题 */
.high-board h4 {
  margin: 0 0 10px 0;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 1px;
}

/* 王区域 */
.joker-row {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--bg-muted, rgba(255, 255, 255, 0.04));
  font-size: 18px;
  font-weight: 550;
}

.joker-row span {
  font-weight: 550;
  font-size: 18px;
}

/* 表格整体 */
.record-table {
  table-layout: fixed;
  width: auto;
  min-width: 300px;
  border-collapse: separate;
  border-spacing: 0;
  margin: 10px 0 0 0;
  text-align: center;
  font-size: 18px;
}

/* 表头 */
.record-table thead th {
  padding: 8px 4px;
  font-weight: 550;
  font-size: 18px;
  color: var(--text-muted, #aaa);
  border-bottom: 2px solid var(--border-muted, rgba(255, 255, 255, 0.1));
}

/* 行 */
.record-table tbody tr {
  transition: background 0.15s ease;
}

.record-table tbody tr:hover {
  background: rgba(255, 255, 255);
}

/* 单元格 */
.record-table td {
  padding: 8px 4px;
  font-weight: 400;
  font-size: 18px;
  border-bottom: 1px solid var(--border-muted, rgba(255, 255, 255, 0.06));
}

/* 花色 */
.suit {
  font-size: 20px;
  font-weight: 400;
}

/* 红黑分色 */
.record-table tbody tr:nth-child(2) .suit,
.record-table tbody tr:nth-child(4) .suit {
  color: #e5484d; /* ♥ ♦ */
}

.record-table tbody tr:nth-child(1) .suit,
.record-table tbody tr:nth-child(3) .suit {
  color: #e5e5e5; /* ♠ ♣ */
}

/* 提示 */
.hint {
  margin-top: 10px;
  font-size: 18px;
  text-align: center;
  color: var(--text-muted, #888);
}
</style>
