本项目是一个基于 Go + WebSocket + Vue 实现的局域网多人在线“升级”游戏。

游戏界面展示

![移动端/PC端界面](images/2.png)


## 文件目录

/前后端启动

    /cmd/server/main.go            后端入口，启动命令：go run ./cmd/server
    /frontend/src/...              前端代码，启动命令：npm --prefix .\frontend run dev


/internal/ws/                  WebSocket 连接层

      hub.go           # 全局ws hub：连接管理、广播
      conn.go          # 单连接读写、心跳、鉴权

/internal/room/                  房间管理（非规则）

      room.go          # 房间生命周期、玩家入座准备
      router.go        # 事件路由：把客户端event送进game reducer
      manager.go       # 房间管理器

/internal/game/

      rules/
          card.go        # 卡牌基本数据结构
          compare.go     # 牌型比较
          deck.go        # 发牌、洗牌
          follow.go      # 跟牌约束
          pattern.go     # 牌域识别（主副牌）、牌型识别（单/对/拖拉机/甩牌）
          score.go       # 分牌计算、末墩抠底倍数、结算升级
          sort.go        # 手牌排序
          trump.go       # 定主/改主/攻主/硬主规则
      error.go         # 错误处理
      events.go        # 客户端、服务端事件
      reducer.go       # 处理核心 (state, event) -> newState + outputs
      snapshot.go      # 客户端消息
      state.go         # 游戏状态
      utils.go         # 工具函数

## 环境说明
    go   1.24       （go version）
    npm  10.9.0     （npm -v）
    node v22.12.0   （node -v）

## 使用说明:
    后端：
      拉取依赖 go mod tidy
      启动后端 go run ./cmd/server
      此时后端默认监听地址：http://localhost:8080
    
    前端：
      进入目录 cd frontend
      安装依赖 npm install
      修改后端地址 将frontend/src/components/ConnectionPanel.vue中的wsBase替换为当前服务器IP
      启动前端 npm --prefix .\frontend run dev

    用户：
      与服务器连接同一局域网
      访问网址 http://{IP地址}:5173/ (通过ipconfig查询服务器IP地址)

## 游戏规则

本游戏按照孝汾地区的民间升级规则开发，详见同级目录下的[简版规则.md](简版规则.md)

与游戏开发相关的状态机设计、游戏阶段流转模型，详见同级目录下的[阶段模型.md](阶段模型.md)

## 后续TODO

连接与回合管理脆弱（panic风险）

    Room 生命周期没有闭环
    GameState 可能被并发访问
    WS 断开没有做状态迁移
    回合切换可能在非法 phase 下执行

    缺少：
    断线重连机制
    Room 的状态机保护
    每个入口必须校验 phase
    panic recover 保护层
    GameState 不允许裸露访问
    禁止外部模块直接修改 state
    所有写操作走 Engine


WS 幂等性（reqId + ack）

    目前如果：客户端重复发送、网络重传、页面刷新、会出现：重复出牌、重复定主、状态错乱
    保存最近 N 个 reqId、已处理直接返回结果、返回 ackReqId
    Snapshot的一致性保证

防御性编程缺失

    slice 越界
    nil map
    非法 seatId
    非法 card id
    phase 不匹配
    
    必须：
    所有入口校验
    所有 index 前检查
    所有 map 初始化明确

    还有一些冗余的防御性编程校验，没有明确划分区域

Engine 封装
  
    问题：Room 依赖 Game、Game 又暴露内部 State、规则 scattered
    需要：Room→Engine→GameState→Rules，外部不允许直接改 state
    Engine 设计目标，只暴露：Query 接口（只读）+ Command 接口（写）

其余问题

    外置配置项
    将一些函数改为类方法
    错误码标准化：进一步区分业务错误、非法请求、系统错误
    遗漏的规则项：同一张王牌和级牌不可多次参与该局的定主、改主、攻主
    缺少系统级后台日志，如replay log。目前debug主要靠在前端game.ts中打印msg
    缺少单元测试
    前端结构混乱
