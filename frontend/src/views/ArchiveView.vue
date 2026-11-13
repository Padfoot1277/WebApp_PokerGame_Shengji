<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import ArchiveCards from "../components/ArchiveCards.vue";
import { seatName } from "../utils/seat";
import type { Card, ViewState } from "../types/view";

type State = Pick<
  ViewState,
  | "roomId"
  | "roundIndex"
  | "roundPointsFinal"
  | "roundResultLabel"
  | "callerDelta"
  | "defenderDelta"
  | "points"
  | "trickIndex"
  | "trick"
  | "trump"
  | "bottomAward"
  | "surrendered"
  | "phase"
> & {
  seats: { name: string; isBot: boolean; team: number }[];
  hands: (Card[] | null)[];
  bottom: Card[] | null;
};
type Action = {
  event: string;
  seat: number;
  payload: Record<string, number | number[]> | null;
  state: State;
};
type Detail = { initial: State; fromDeal: boolean; actions: Action[] };
type Entry = {
  id: string;
  room: string;
  number: number;
  points: number;
  result: string;
  surrendered: boolean;
};
const props = defineProps<{ id: string }>();
const search = ref(""),
  filter = ref(""),
  cursors = ref([0]),
  page = computed(() => cursors.value.length);
const cursor = computed(() => cursors.value[cursors.value.length - 1] ?? 0);
const refresh = ref(0),
  next = ref(0),
  entries = ref<Entry[]>([]);
const detail = shallowRef<Detail | null>(null),
  error = ref(""),
  loading = ref(true),
  step = ref(0);
watch(
  () => [props.id, filter.value, cursor.value, refresh.value],
  async (_, __, cleanup) => {
    const controller = new AbortController();
    cleanup(() => controller.abort());
    loading.value = true;
    error.value = "";
    detail.value = null;
    entries.value = [];
    next.value = 0;
    step.value = 0;
    try {
      const params = new URLSearchParams();
      if (filter.value) params.set("room", filter.value);
      if (cursor.value) params.set("before", String(cursor.value));
      const response = await fetch(
        props.id
          ? "/api/rounds/" + encodeURIComponent(props.id)
          : "/api/rounds?" + params,
        { signal: controller.signal },
      );
      if (!response.ok)
        throw Error(
          response.status === 404 ? "这份档案不存在。" : "暂时无法读取档案。",
        );
      const data = await response.json();
      if (controller.signal.aborted) return;
      if (props.id) detail.value = data;
      else {
        entries.value = data.items;
        next.value = data.next;
      }
    } catch (e) {
      if (!controller.signal.aborted)
        error.value = e instanceof Error ? e.message : "加载失败";
    } finally {
      if (!controller.signal.aborted) loading.value = false;
    }
  },
  { immediate: true },
);
function find() {
  filter.value = search.value.trim();
  cursors.value = [0];
  refresh.value++;
}
const labels: Record<string, string> = {
  "room.ready": "准备并发牌",
  "game.start": "开始小局",
  "game.start_next_round": "开始下一小局",
  "game.call_trump": "定主",
  "game.call_pass": "跳过",
  "game.change_trump": "改主",
  "game.attack_trump": "攻主",
  "game.put_bottom": "扣底",
  "game.play_cards": "出牌",
  "game.surrender": "确认弃局",
  "game.cancel_surrender": "撤回弃局",
};
const phases: Record<string, string> = {
  call_trump: "定主",
  bottom: "扣底",
  trump_fight: "攻改",
  play_trick: "出牌",
  round_settle: "小局结算",
  game_over: "整场结束",
};
const action = computed(() => detail.value?.actions[step.value - 1]);
const state = computed(() => action.value?.state ?? detail.value?.initial);
const final = computed(
  () =>
    detail.value?.actions[detail.value.actions.length - 1]?.state ??
    detail.value?.initial,
);
const cards = computed(() => {
  const index = new Map<number, Card>();
  const s = detail.value?.initial;
  for (const c of [
    ...(s?.hands.flatMap((h) => h ?? []) ?? []),
    ...(s?.bottom ?? []),
  ])
    index.set(c.id, c);
  return index;
});
const requested = computed(() => {
  const p = action.value?.payload;
  return Object.values(p ?? {})
    .flat()
    .map((id) => cards.value.get(id) ?? null);
});
const previous = computed(
  () => detail.value?.actions[step.value - 2]?.state ?? detail.value?.initial,
);
const resolved = computed(
  () =>
    !!state.value &&
    !!previous.value &&
    state.value.trickIndex > previous.value.trickIndex,
);
const plays = computed(() =>
  resolved.value
    ? state.value?.trick.lastMoves
    : state.value?.trick.playedMoves,
);
function actor(s: State, seat: number) {
  return s.seats[seat]?.name || seatName(seat);
}
</script>
<template>
  <main class="archives">
    <nav>
      <a v-if="id" href="#/archives">← 档案列表</a
      ><a v-else href="#">← 返回牌桌</a>
    </nav>
    <header class="mast">
      <div>
        <h1>{{ id ? "小局复盘" : "小局档案" }}</h1>
        <p v-if="!id">按房间查找已结算的小局，回看每一步出牌。</p>
        <p v-else-if="final">
          房间 {{ final.roomId }} · 第 {{ final.roundIndex + 1 }} 小局
        </p>
      </div>
      <span class="suits" aria-hidden="true">♠ <i>♥</i> ♣ <i>♦</i></span>
    </header>
    <form v-if="!id" @submit.prevent="find">
      <label for="archive-room">房间码</label
      ><input
        id="archive-room"
        v-model="search"
        type="search"
        maxlength="32"
        placeholder="全部房间"
      /><button>查询</button>
    </form>
    <div v-if="loading" class="notice" role="status">正在读取档案…</div>
    <div v-else-if="error" class="notice" role="alert">
      {{ error }} <button class="secondary" @click="refresh++">重试</button>
    </div>
    <template v-else-if="!id">
      <section v-if="entries.length" class="listing">
        <a v-for="e in entries" :key="e.id" :href="'#/archives/' + e.id">
          <div>
            <small>房间 {{ e.room }}</small>
            <h2>第 {{ e.number }} 小局</h2>
          </div>
          <div class="outcome">
            <strong>{{ e.points }}</strong> 分
            <span>{{ e.result }}{{ e.surrendered ? " · 弃局" : "" }}</span>
          </div>
          <b aria-hidden="true">→</b>
        </a>
      </section>
      <div v-else class="notice">
        <h2>{{ filter ? "未找到这个房间的档案" : "暂无已结算的小局" }}</h2>
        <p>
          {{
            filter
              ? "检查房间码，或清空输入框查看全部房间。"
              : "小局结算并确定打家得分后，会显示在这里。"
          }}
        </p>
      </div>
      <footer v-if="next || page > 1">
        <button class="secondary" :disabled="page === 1" @click="cursors.pop()">
          上一页</button
        >第 {{ page }} 页<button
          class="secondary"
          :disabled="!next"
          @click="cursors.push(next)"
        >
          下一页
        </button>
      </footer>
    </template>
    <template v-else-if="detail && state && final">
      <section class="summary">
        <div>
          <small>打家最终得分</small>
          <div>
            <strong>{{ final.roundPointsFinal }}</strong> 分
          </div>
        </div>
        <div>
          <h2>
            {{ final.roundResultLabel }} {{ final.surrendered ? "· 弃局" : "" }}
          </h2>
          <p>
            坐家升 {{ final.callerDelta }} 级 · 打家升
            {{ final.defenderDelta }} 级
          </p>
          <small>抠底得分 {{ final.bottomAward }}</small>
        </div>
      </section>
      <section class="review">
        <div class="headline">
          <h2>逐步回放</h2>
          <small>{{ step }} / {{ detail.actions.length }}</small>
        </div>
        <p v-if="!detail.fromDeal">旧版存档，从恢复时的牌面开始回放。</p>
        <div class="controls">
          <button class="secondary" :disabled="!step" @click="step--">
            上一步
          </button>
          <select v-model.number="step" aria-label="跳转到操作">
            <option :value="0">
              {{ detail.fromDeal ? "发牌后" : "初始快照" }}
            </option>
            <option v-for="(a, i) in detail.actions" :key="i" :value="i + 1">
              {{ i + 1 }} · {{ actor(a.state, a.seat) }} ·
              {{ labels[a.event] ?? a.event }}
            </option>
          </select>
          <button
            class="secondary"
            :disabled="step >= detail.actions.length"
            @click="step++"
          >
            下一步
          </button>
        </div>
        <article class="tableau">
          <div class="headline">
            <h3>{{ phases[state.phase] ?? state.phase }}</h3>
            <span
              >打家累计 <b>{{ state.points }}</b> 分</span
            >
          </div>
          <p v-if="action">
            {{ seatName(action.seat) }} · {{ actor(state, action.seat) }} ·
            {{ labels[action.event] ?? action.event }}
          </p>
          <p v-if="requested.length">
            提交的牌 <ArchiveCards :cards="requested" />
          </p>
          <p v-if="action?.event === 'game.play_cards'">
            第 {{ (previous?.trickIndex ?? 0) + 1 }} 墩
            <span v-if="resolved">
              · {{ seatName(state.trick.winnerSeat) }} 赢墩</span
            >
          </p>
          <div v-if="action?.event === 'game.play_cards'" class="played">
            <div v-for="(move, i) in plays" :key="i">
              <small>{{ seatName(i) }}</small
              ><ArchiveCards :cards="move?.followMove.cards" />
            </div>
          </div>
        </article>
      </section>
      <section class="review">
        <div class="headline">
          <h2>四家手牌</h2>
          <small>{{
            state.trump.levelRank === "Pending"
              ? "待定主"
              : (state.trump.hasTrumpSuit ? state.trump.suit : "无主花色") +
                " · 级牌 " +
                state.trump.levelRank
          }}</small>
        </div>
        <div class="hand" v-for="(s, i) in state.seats" :key="i">
          <div>
            <b>{{ seatName(i) }} · {{ actor(state, i) }}</b
            ><small
              >{{ s.isBot ? "机器人 · " : ""
              }}{{ state.hands[i]?.length ?? 0 }} 张</small
            >
          </div>
          <ArchiveCards :cards="state.hands[i]" />
        </div>
        <div class="hand">
          <b>底牌</b><ArchiveCards :cards="state.bottom" />
        </div>
      </section>
    </template>
  </main>
</template>
<style scoped>
.archives {
  width: min(100%, 920px);
  padding: 20px clamp(14px, 4vw, 42px) 64px;
  margin: 0 auto;
  color: #193d32;
}
nav {
  margin-bottom: 18px;
}
nav a {
  color: #3e6857;
  text-underline-offset: 4px;
}
.mast,
.summary,
.headline,
.controls,
form,
footer,
.listing a,
.hand > div {
  display: flex;
  align-items: center;
  gap: 12px;
}
.mast,
.headline,
.hand > div {
  justify-content: space-between;
}
.mast {
  background: #163e32;
  color: #fffaf0;
  padding: clamp(22px, 4vw, 36px);
  border-radius: 14px;
  border-bottom: 4px solid #b89557;
}
h1 {
  font:
    700 clamp(28px, 4vw, 40px)/1.3 Georgia,
    "Songti SC",
    serif;
}
.mast p {
  color: #e0e8de;
  margin-top: 8px;
}
.suits {
  color: #cfb787;
  white-space: nowrap;
  letter-spacing: 3px;
  font-size: 20px;
}
.suits i {
  color: #d69784;
  font-style: normal;
}
form {
  margin: 22px 0;
}
input,
select {
  min-width: 0;
  padding: 10px;
  border: 1px solid #afc4b5;
  border-radius: 7px;
  font: inherit;
  background: white;
  color: inherit;
}
input {
  flex: 1;
}
.listing,
.notice,
.summary,
.review {
  background: #fffef9;
  border: 1px solid #d7dfd3;
  border-radius: 12px;
}
.listing a {
  padding: 16px 20px;
  color: inherit;
  text-decoration: none;
}
.listing a + a,
.hand + .hand {
  border-top: 1px solid #dde2d6;
}
.listing a:hover {
  background: #f0f3e9;
}
.listing h2,
.headline h2,
.headline h3 {
  margin: 0;
}
.outcome {
  margin-left: auto;
  text-align: right;
}
.outcome span,
small {
  display: block;
  color: #52685d;
}
.outcome strong,
.summary strong {
  font:
    700 32px/1.1 Georgia,
    serif;
}
.notice {
  padding: 32px 20px;
  text-align: center;
}
.summary,
.review {
  margin: 16px 0;
  padding: 20px;
}
.summary {
  gap: 32px;
}
.summary strong {
  font-size: 44px;
}
.summary h2 {
  margin: 0;
}
.controls {
  margin: 16px 0;
}
.controls select {
  flex: 1;
}
.tableau {
  padding: 16px;
  background: #eaf0e6;
  border-radius: 8px;
}
.tableau p {
  margin-top: 8px;
}
.played {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}
.hand {
  padding: 12px 0;
}
.hand > div {
  margin-bottom: 6px;
}
.hand:last-child {
  padding-bottom: 0;
}
footer {
  justify-content: center;
  margin-top: 16px;
}
@media (max-width: 520px) {
  .suits {
    display: none;
  }
  .controls {
    gap: 5px;
  }
  .controls button {
    padding: 8px;
  }
  .summary,
  .review {
    padding: 14px;
  }
  .listing a {
    padding: 12px;
  }
  .listing h2 {
    font-size: 18px;
  }
  .mast {
    padding: 20px;
  }
  .hand b,
  .mast p,
  .listing small {
    overflow-wrap: anywhere;
  }
}
</style>
