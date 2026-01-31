/cmd/server/main.go                // 启动入口

/internal/ws/                  // websocket 连接层

      hub.go           # 全局ws hub：连接管理、广播
      conn.go          # 单连接读写、心跳、鉴权
      protocol.go      # 消息格式、编解码

/internal/room/                    // 房间管理（非规则）

      room.go          # 房间生命周期、玩家入座准备
      router.go        # 事件路由：把客户端event送进game reducer

/internal/game/                     

      state.go         # GameState + 子结构
      phase.go         # Phase定义 + 允许事件表
      events.go        # ClientEvent / ServerEvent
      reducer.go       # Reduce(state, event) -> newState + outputs
      rules/
        deck.go        # 发牌、洗牌、底牌
        trump.go       # 定主/改主/攻主/硬主
        pattern.go     # 牌型识别：单/对/拖拉机/甩牌
        follow.go      # 跟牌约束、是否可杀、是否垫牌
        compare.go     # 一墩胜负比较（含主/副、级牌）
        score.go       # 分牌计算、末墩抠底倍数、结算升级
      util/
        id.go 
        rand.go  

/web/                              // 前端静态资源

    index.html
    app.js
    net/ws.js            # websocket收发 + 重连
    store/state.js       # 当前room视图状态（服务器下发）
    ui/
        lobby.js
        table.js           # 四人座位、状态提示
        hand.js            # 手牌渲染/选择/排序
        action.js          # 按钮区：准备/定主/扣底/出牌...
