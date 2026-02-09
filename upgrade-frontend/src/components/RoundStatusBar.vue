<script setup lang="ts">
import { computed } from 'vue'
import { useGameStore } from '../store/game'

const game = useGameStore()
const v = computed(() => game.view)

const phase = computed(() => v.value?.phase)
const trump = computed(() => v.value?.trump)
const bottomOwner = computed(() => v.value?.bottomOwnerSeat)

const phaseText: Record<string, string> = {
  lobby: '等待入座/准备',
  dealing: '发牌中',
  call_trump: '定主/抢主',
  bottom: '收底/扣底',
  trump_fight: '改主/攻主窗口',
  play_trick: '出牌中',
  follow_trick: '跟牌中',
}


</script>

<template>
  <div class="panel">
    <div>阶段：{{ phaseText[phase] ?? phase }}</div>

    <div v-if="trump">
      <span>主牌：</span>
      <span v-if="trump.hasTrumpSuit">
        {{ trump.suit }} ｜ 级牌 {{ trump.levelRank }}
      </span>
      <span v-else>硬主</span>

      <span v-if="trump.locked">（锁主）</span>
    </div>

    <div v-if="bottomOwner >= 0">
      坐家：Seat {{ bottomOwner }}
    </div>
  </div>
</template>
