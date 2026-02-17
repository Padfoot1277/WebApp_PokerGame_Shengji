<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'
import { suitToSymbol } from '../utils/card'

const game = useGameStore()
const v = computed(() => game.view)

const phase = computed(() => v.value?.phase ?? '')
const trump = computed(() => v.value?.trump)
const starterSeat = computed(() => v.value?.starterSeat ?? -1)
const beaterScore = computed(() => v.value?.points ?? -1)

const phaseText: Record<string, string> = {
  lobby: '准备',
  dealing: '发牌',
  call_trump: '定主',
  bottom: '扣底',
  trump_fight: '攻改',
  play_trick: '出牌',
  follow_trick: '跟牌',
  round_settle: '小局结束',
}
const nextRoundOwner = computed(() => v.value?.nextStarterSeat ?? -1)

const trumpSuitInfo = computed(() => {
  if (!trump.value?.hasTrumpSuit) return null
  return suitToSymbol(trump.value.suit)
})

function seatLabel(idx: number): string {
  const map = ['⓪', '①', '②', '③']
  return map[idx] ?? String(idx)
}
</script>

<template>
  <div class="panel">
    <div class="info-line">
  <span class="tag">
    <strong>阶段</strong>
    {{ phaseText[phase] ?? phase }}
  </span>

      <template v-if="starterSeat >= 0">
    <span class="tag">
      <strong>坐庄</strong>
      {{ seatLabel(starterSeat) }}
    </span>

        <span class="tag">
          <strong>级牌</strong> {{ trump.levelRank }}
    </span>

        <span
            v-if="trump.levelRank !== 'Pending'"
            class="tag"
        >
      <strong>主牌</strong>
      <span
          v-if="trump.hasTrumpSuit && trumpSuitInfo"
          :class="{ locked: trump.locked }"
      >
          {{ trumpSuitInfo.symbol }}
      </span>
      <span v-else class="trump-badge hard">硬主</span>
    </span>

        <span v-if="trump.locked" class="tag warn">
      🔒锁主
    </span>

        <span
            v-if="phase === 'round_settle' || phase === 'play_trick'"
            class="tag score"
        >
      <strong>得分</strong> {{ beaterScore }}
    </span>
      </template>
    </div>


    <div
        v-if="phase === 'round_settle'"
        class="row"
    >
  <span v-if="nextRoundOwner >= 0">
    请等待 {{ nextRoundOwner }}号位 开始下一小局
  </span>
    </div>
  </div>
</template>

<style scoped>
.row {
  margin-top: 6px;
}

.ml {
  margin-left: 8px;
}
.level {
  margin-left: 10px;
}

/* 花色上色：黑桃/梅花纯黑，红桃/方块纯红 */
.suit.black {
  color: #000; /* 纯黑（如果太暗可改 #111） */
}

.suit.red {
  color: #e53935;
}

.suit.joker {
  color: #ffd54f;
}

/* 主牌徽章（白底，突出显示） */
.trump-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;

  min-width: 28px;
  height: 28px;
  margin-left: 4px;

  background: #ffffff;
  border-radius: 6px;
  border: 1px solid #ccc;

  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
}

/* 花色本身略放大一点 */
.trump-badge .suit {
  font-size: 18px;
  line-height: 1;
}

/* 继承你已有的颜色规则 */
.suit.black {
  color: #000;
}

.suit.red {
  color: #e53935;
}

.suit.joker {
  color: #ffd54f;
}

.trump-badge {
  border-color: #4da3ff;   /* 用你已有的 notice 蓝 */
}

.trump-badge.locked {
  box-shadow: 0 0 0 2px rgba(77, 163, 255, 0.5);
}

.trump-badge.hard {
  font-size: 12px;
  color: #333;
}

.info-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap; /* 屏幕窄时自动换行 */
  gap: 8px;
  font-size: 13px;
}

/* 基础胶囊 */
.tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;

  padding: 2px 10px;
  border-radius: 999px;

  background: #f3f4f6;
  color: #333;
  line-height: 20px;
  white-space: nowrap;
  font-size: 14px; /* 原来 13px，改大一点即可 */
}

.tag strong {
  font-weight: 600;
  color: #555;
}

/* 特殊语义 */
.tag.warn {
  background: #fff1f0;
  color: #cf1322;
}

.tag.score {
  background: #f6ffed;
  color: #237804;
}
.tag.score strong {
  color: #237804;
  font-weight: 600;
}


/* 主牌徽章可稍微缩小一点 */
.trump-badge {
  margin-left: 2px;
  display: inline-flex;
  align-items: center;
}

</style>
