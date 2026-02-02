
# 文件目录

/cmd/server/main.go             // 启动入口(go run ./cmd/server)

/internal/ws/                  // websocket 连接层

      hub.go           # 全局ws hub：连接管理、广播
      conn.go          # 单连接读写、心跳、鉴权

/internal/room/                    // 房间管理（非规则）

      room.go          # 房间生命周期、玩家入座准备
      router.go        # 事件路由：把客户端event送进game reducer

/internal/game/

      rules/
        card.go        # 卡牌基本数据结构
        compare.go     # 牌型比较
        deck.go        # 发牌、洗牌
        follow.go      # 跟牌约束、垫牌、主杀判断
        pattern.go     # 牌型识别：单/对/拖拉机/甩牌
        score.go       # 分牌计算、末墩抠底倍数、结算升级
        sort.go        # 手牌排序
        trump.go       # 定主/改主/攻主/硬主
      events.go        # 客户端、服务端事件
      reducer.go       # 处理核心 (state, event) -> newState + outputs
      snapshot.go      # 客户端消息
      state.go         # 游戏状态
      utils.go         # 工具函数

/web/                              // 前端静态资源

    index.html
    app.js


# 一、整体工程架构（已完成）

### 1. 技术选型与总体架构

- **后端**：Go
    - WebSocket 长连接
    - 房间（Room）级别的事件循环
    - reducer + state 的确定性状态机
- **前端**：原生 HTML + JS
    - WebSocket 客户端
    - 纯状态驱动渲染（snapshot → render）
- **通信模型**：
    - ClientEvent → Server reducer → 更新 GameState → 广播 ViewState
    - 明确区分“私有信息（手牌）”与“公共信息（阶段、出牌、seat 状态）”

---

# 二、GameState / 核心状态设计（已完成）

### 2. GameState 核心字段

当前已具备完整的小局级状态表达能力，包括：

- **玩家与座位**
    - Seats（4 座位）
    - seat ↔ uid 映射
    - team / hand / handCount
- **牌堆结构**
    - 手牌 Hand
    - 底牌 Bottom
- **主牌系统**
    - Trump（主花色、是否硬主、级牌）
    - 支持定主 / 改主 / 攻主
- **阶段流转**
    - Lobby / Ready
    - Deal
    - PhaseTrump（抢定主）
    - PhaseBottom（坐家扣底）
    - PhaseTrumpFight（攻主/改主）
    - PhasePlayTrick（出牌回合）
- **回合状态**
    - TrickState
        - LeaderSeat
        - TurnSeat
        - LeadMove（先手）
        - FollowMove（预留）

> 已经从“流程驱动”升级为显式状态机模型。
>

---

# 三、牌与规则引擎（已完成的高难度部分）

### 3. Card / Block / Move 体系

你已经完成了一个**非常扎实、工程级的牌型与比较体系**：

### Card

- 支持：
    - 普通牌 / 大小王
    - Suit / Rank / Color
    - SuitClass（主/副花色域）

### Block（基础牌型）

- single
- pair
- tractor（连续对子）

### Move

- 支持：
    - 多基础牌型组合（甩牌）
    - IntentMove（原意）
    - ActualMove（裁剪后）

---

### 4. 规则核心算法（已完成）

### （1）牌力与比较

- `cardStrength`
- `compareTwoCard`
- 同 SuitClass 内强度完全定义清楚
- 覆盖：王 / 主级 / 副级 / 主花色 / 副花色

### （2）出牌解析

- `DecomposeThrow`
    - 复用 `buildPairs`
    - 贪心生成**尽可能长的拖拉机**
    - 其余拆为对子 / 单张
- 输出结构清晰、稳定、可预测

### （3）甩牌裁剪（非常关键）

- `canonicalizeLead`
    - 对甩牌拆分出的每种基础牌型
    - 在其余三家手牌中查找可压制的最大牌型
    - 一旦发现可压制：
        - 判定甩牌失败
        - 仅保留**最小失败牌型**
        - 其余撤回
- 完整实现了你一开始设计的规则思想

👉 **这一部分已经是升级游戏最难的规则之一，你已经攻克。**

---

# 四、出牌阶段（已完成）

### 5. PhasePlayTrick（先手出牌）

当前已完整支持：

- 先手规则：
    - 第一回合：定主者
    - 后续回合：上一回合胜者（接口已预留）
- 出牌校验：
    - 是否轮到你
    - 是否已出过牌
    - 所选牌是否在手
    - 是否同一 SuitClass
- 支持：
    - 单张 / 对子 / 拖拉机
    - 甩牌（含自动裁剪）
- 状态写入：
    - TrickState.Lead
    - TurnSeat 推进
    - 手牌扣除
- 完整错误保护（非法出牌不污染状态）

---

# 五、前端交互与展示（已完成）

### 6. 手牌 / 底牌 UI

- 手牌、底牌分别展示
- 使用 emoji 显示：
    - 大小王
    - 花色
    - 红黑色区分
- 支持点击选择 / 取消选择
- 阶段切换时自动清空选择（失败不清空）

---

### 7. seatBar（核心 UI 已重构完成）

你已经完成了一个**非常合理的展示方案**：

- 每个 Seat 独立卡片
- 显示：
    - 🟦 自己
    - 🎯 本回合先手
    - ⏳ 当前轮到谁
    - 🟨 坐家
- **本回合出牌直接显示在 seat 卡片内**
    - 不再使用独立 leadPanel
- 甩牌失败：
    - 明显红色提示
    - 可选展示原甩牌意图（置灰）

👉 这个 UI 结构非常适合后续“跟牌”和“回合结算”。

---

# 六、当前系统能力边界（非常重要）

### ✅ 已完成

- 四人升级游戏完整 **前半流程**
- 定主 / 改主 / 攻主 / 扣底
- 先手出牌 + 甩牌裁剪（核心难点）
- 状态机 + UI + 规则引擎基本成型

### ❌ 尚未完成（但已具备承载能力）

- 跟牌（FollowMove 合法性）
- 回合胜负判定
- 回合得分（吃分）
- 末墩抠底
- 小局结算 / 升级
- 整局结束判定