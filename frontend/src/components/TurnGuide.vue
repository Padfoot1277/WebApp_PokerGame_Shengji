<script setup lang="ts">
import { computed } from "vue";
import { useGameStore } from "../store/game";
import { relativeSeat, seatName } from "../utils/seat";
const game = useGameStore();
const guide = computed(() => {
  const s = game.view;
  if (!s) return { title: "正在同步牌局", detail: "" };
  if (s.mySeat < 0)
    return {
      title: "正在观战",
      detail:
        "当前牌局已开始，暂不能入座。你可以继续观战，或返回大厅创建新房间。",
    };
  const who = (seat: number) =>
    `${relativeSeat(seat, s.mySeat)} · ${s.seats[seat]?.name || seatName(seat)}`;
  switch (s.phase) {
    case "call_trump": {
      const can =
        !s.callPassedSeats[s.mySeat] &&
        (s.callMode === "race" || s.callTurnSeat === s.mySeat);
      return {
        title: can
          ? "现在可以定主"
          : s.callPassedSeats[s.mySeat]
            ? "你已跳过定主"
            : `等待${who(s.callTurnSeat)}定主`,
        detail: can
          ? `选择一张王 + 本队级牌 ${s.teams[s.mySeat % 2]?.levelRank}；同色王配同花色一对级牌可锁主。不定主可跳过，本窗口不可反悔。`
          : "其他人正在选择，主花色确定后继续。",
      };
    }
    case "bottom":
      return {
        title:
          s.bottomOwnerSeat === s.mySeat
            ? "请从 33 张手牌中扣下 8 张"
            : `等待${who(s.bottomOwnerSeat)}扣底`,
        detail:
          s.bottomOwnerSeat === s.mySeat
            ? "底牌已经加入手牌。选满 8 张后提交，留下 25 张用于出牌。"
            : "扣底完成后，从收底者下一席依次攻改。",
      };
    case "trump_fight":
      return {
        title:
          s.fightTurnSeat === s.mySeat
            ? "轮到你改主 / 攻主"
            : `等待${who(s.fightTurnSeat)}攻改`,
        detail: s.trump.locked
          ? "主花色已锁定：只能用一对同类王攻主，或选择跳过。"
          : "改主选一张同色王 + 一对本局级牌；攻主选一对同类王。已亮主的牌不可重用。",
      };
    case "play_trick": {
      const lead = s.trick.playedMoves[s.trick.leaderSeat];
      const mine = s.trick.turnSeat === s.mySeat;
      return {
        title: mine
          ? lead
            ? `请跟出 ${lead.followMove.cards.length} 张${lead.suitClass}`
            : "轮到你先手出牌"
          : `等待${who(s.trick.turnSeat)}出牌`,
        detail: lead
          ? "有先手牌域必须优先跟出，有对子 / 拖拉机须尽量满足结构；牌域不足才可补牌。"
          : "可以出同一牌域的单张、对子、拖拉机或甩牌。金色边框标记当前行动者。",
      };
    }
    case "round_settle":
      return {
        title: "本小局已结束",
        detail: `${who(s.nextStarterSeat)}拥有下一局开始权。可先查看本局得分和升级结果。`,
      };
    case "game_over":
      return {
        title: "整局结束",
        detail: "已有队伍升到 A。返回大厅创建新房间，即可重新开始。",
      };
    default:
      return { title: "等待牌友准备", detail: "" };
  }
});
const offline = computed(() =>
  game.view?.seats
    .filter((s) => s.uid && !s.online)
    .map((s) => s.name)
    .join("、"),
);
</script>
<template>
  <section class="guide" aria-live="polite">
    <strong>{{ guide.title }}</strong>
    <p>{{ guide.detail }}</p>
    <p v-if="offline" class="offline">
      {{ offline }}暂时离线，座位与手牌已保留，重连后可继续。
    </p>
  </section>
</template>
<style scoped>
.guide {
  background: #e7f0e8;
  border-left: 4px solid #26765c;
  border-radius: 10px;
  padding: 12px 16px;
}
.guide strong {
  font-size: 18px;
}
.guide p {
  font-size: 13px;
  color: #52695b;
  margin-top: 4px;
}
.guide .offline {
  color: #a05c25;
}
</style>
