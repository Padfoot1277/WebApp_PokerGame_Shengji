<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'
import { suitToSymbol } from '../utils/card'

const game = useGameStore()
const v = computed(() => game.view)

const phase = computed(() => v.value?.phase ?? '')
const trump = computed(() => v.value?.trump)
const bottomOwner = computed(() => v.value?.bottomOwnerSeat ?? -1)

const phaseText: Record<string, string> = {
  lobby: '等待入座/准备',
  dealing: '发牌中',
  call_trump: '定主/抢主',
  bottom: '收底/扣底',
  trump_fight: '改主/攻主窗口',
  play_trick: '出牌中',
  follow_trick: '跟牌中',
}

const trumpSuitInfo = computed(() => {
  if (!trump.value?.hasTrumpSuit) return null
  return suitToSymbol(trump.value.suit)
})
</script>

<template>
  <div class="panel">
    <div>阶段：{{ phaseText[phase] ?? phase }}</div>

    <div v-if="trump" class="row">
      <span>级牌：{{ trump.levelRank }}</span>

      <span class="ml">
        主牌：
<span
    v-if="trump.hasTrumpSuit && trumpSuitInfo"
    class="trump-badge"
    :class="{ locked: trump.locked }"
>
  <span class="suit" :class="trumpSuitInfo.color">
    {{ trumpSuitInfo.symbol }}
  </span>
</span>

<span v-else class="trump-badge hard">
  硬主
</span>
      </span>

      <span v-if="trump.locked" class="ml">（锁主）</span>
    </div>

    <div v-if="bottomOwner >= 0">
      坐家：Seat {{ bottomOwner }}
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

</style>
