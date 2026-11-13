# 升级（孝汾规则）

四人固定对家、两副牌的在线扑克游戏。后端使用 Go，前端使用 Vue 3、TypeScript、Pinia 和 Vite。规则由服务端判定，客户端只接收自己的手牌。

## 本地运行

环境：Go 1.24+、Node.js 22.12+。

```powershell
npm --prefix frontend ci
npm --prefix frontend run build
go run ./cmd/server
```

浏览器访问 http://localhost:8080 。创建房间后分享房间码；四名玩家各自入座、准备后开局。同一局域网的设备使用服务器的 IP 地址访问 8080 端口。

开发模式另开终端运行 `npm --prefix frontend run dev`，访问 5173 端口；Vite 自动代理 `/ws` 到 8080，无需修改源码中的 IP。

## 验证

```powershell
go test ./...
go vet ./...
npm --prefix frontend run build
npm --prefix frontend test
npm --prefix frontend run format:check
```

GitHub Actions 还会在 Linux 上运行 `go test -race ./...`。本机未启用 CGO 时无法执行竞态检测。

## 功能与协议

- 网页和微信小游戏采用相同信息顺序与座位顺序；小游戏请求竖屏，长手牌分花色独立滚动；网页手牌随页面自然展开，不设内部滚动和额外工具行。两端支持点选、横向滑选，纵向滑动不误选。
- 大厅座位使用 A1、B1、A2、B2 标识；A组和B组分坐对家位。牌桌按玩家视角显示我、上家、对家、下家。
- 准备界面支持增加/减少机器人：每次填补一个空位，减少时先添加先移除；开局后不可增减。机器人由服务端直接驱动，不占 WebSocket 连接，自动准备、定主、攻改、扣底、出牌及开始下一小局。定主能定就定，优先牌多的花色；攻改能攻优先攻，否则能改就改。真人离开后暂停自动行动。正式机器人使用服务器内全局手牌快照做有界的一墩团队推演：打家争取吃分，坐家优先安全跑分；考虑让牌保大、逼对手花大牌、短门及小调主交权。扣底比较清空短副牌门与保留 A、大对子等控制牌的收益。额外手牌和底牌仅供服务器决策，不发送给真人或观战客户端。
- 支持定主、锁主、按序攻改、硬主、对子、拖拉机、甩牌、跟牌校验、记牌、抠底、升级与升 A 获胜。
- 同队两人确认弃局后，按当时已结算的打家得分立即结算，不额外抠底。第一人可在第二人确认前撤回。
- 昵称与身份分离。服务端签发会话凭据，浏览器标签页使用 sessionStorage 保存；刷新和网络重连可恢复原座位。新标签页可作为另一位玩家。
- 请求包含 `type`、`payload`、`reqId`；响应 `ack` 包含 `reqId`、`ok`、`code`、`message`、`version`。每个玩家每房间保留最近 128 条回执，重复请求返回原回执与最新快照。
- 客户端一次只提交一个待确认请求；自动重连时使用原编号重试，避免重复执行。
- 有限消息大小、请求频率、每房间连接数、发送背压处理及同源检查。无连接的房间在最后真人信号30分钟后从内存回收；最后真人进入或操作超过24小时后，清除该房间的牌局与回执，断开残留连接，同房间号再次进入即为新游戏。机器人动作和自动心跳不续期，服务器启动及每分钟清理未加载的过期存档。诊断日志、会话身份密钥不随单个房间删除。

## 配置与保存

| 环境变量 | 默认值 | 用途 |
| --- | --- | --- |
| `ADDR` | `:8080` | 服务监听地址 |
| `WEB_DIR` | `./frontend/dist` | 静态前端目录 |
| `DATA_DIR` | `./data` | 牌局、回执、会话签名密钥和诊断日志 |
| `SESSION_SECRET` | 自动保存的随机密钥 | 可选，至少 32 字符；固定后可跨重启验证会话 |
| `ALLOWED_ORIGINS` | 仅同源 | 额外允许的完整 Origin，以逗号分隔 |
| `VITE_WS_URL` | 当前站点 `/ws` | 构建时可指定独立 WebSocket 地址 |

房间快照与回执保存在 `DATA_DIR/rooms.sqlite`。每次有效操作与对应的小局记录在同一个 SQLite 事务中提交。服务重启后按房间码加载保存状态，真人玩家初始离线并取消准备，重新连接恢复；机器人保留身份、添加顺序并由服务器继续驱动，公开出牌记忆从当前可见的上一墩重新积累。

`data` 含完整手牌、底牌、身份签名密钥和操作日志，属于服务器私有数据，不对外提供静态访问，也不提交到 Git。请使用同一个数据目录与密钥重启服务。当前文件保存面向单进程部署，不支持多个服务实例同时写同一目录。诊断日志尚未实现完整可视化复盘及保留周期管理。

### 小局档案与训练数据

服务端从发牌起在房间库暂存每小局的牌面和操作，打家最终得分确定后，将整局一次性写入 `DATA_DIR/archive/rounds.sqlite`。房间过期、同号重建和“更新服务器并清空牌局”都保留档案。未结算时可以重连续打；房间过期或被清空时，未结算小局的暂存记录随房间删除。升级时旧档案中的未结算小局也按这一规则处理。数据库随服务端备份，可用 SQLite 工具只读查询。

- `rounds` 每小局一行，包含房间号、从 0 开始的小局序号、结算阶段和初始牌面。旧版房间首次迁移时以现存快照开始，标记为 `from_deal=0`。
- `actions` 按 `round_id, version` 排序，记录操作者座位、动作、请求牌 ID 和生效后的完整牌面。开局动作对应发牌后的快照。版本号因重连等事件可以不连续。每个快照的 `hands` 和 `bottom` 保存四家手牌与底牌，`seats[].isBot` 标识服务端机器人。通过普通 WebSocket 接入的外部陪练和真人使用相同的客户端协议。
- 训练时可用动作前的本家手牌与当时公开的信息作为输入，用请求和实际出牌比较甩牌意图与裁剪结果。以下查询选出从发牌开始、已经结算的小局及其得分、弃局标志。同队两人确认弃局并结算得分的样本也会归档。

```sql
SELECT r.id, r.room, r.round_index,
       json_extract(a.state_after, '$.roundResultLabel') AS result,
       json_extract(a.state_after, '$.roundPointsFinal') AS points,
       json_extract(a.state_after, '$.surrendered') AS surrendered
FROM rounds AS r JOIN actions AS a ON a.round_id = r.id
WHERE r.from_deal = 1 AND r.phase IN ('round_settle', 'game_over')
  AND a.version = (SELECT MAX(version) FROM actions WHERE round_id = r.id);
```

公网部署需要另行配置 HTTPS/WSS 和反向代理。当前同时提供 Web 和竖屏微信小游戏客户端；正式微信登录及正式微信发布仍需后续接入。

## 项目结构

- `cmd/server`：HTTP 与 WebSocket 服务入口。
- `internal/ws`：签名身份、连接、读写循环、连接接管。
- `internal/bot`：复用的机器人决策、公开记牌、定主攻改、扣底与出牌策略。
- `internal/room`：串行房间事件循环、协议解析、回执去重、本地保存。
- `internal/game`：阶段状态机、命令边界校验、弃局、结算、私有快照。
- `internal/game/rules`：牌型、比较、跟牌、洗牌与分值。
- `frontend/src`：Vue 界面、强类型协议、连接状态和交互。

规则见 [简版规则.md](简版规则.md)，状态流转见 [阶段模型.md](阶段模型.md)，本轮实施与验证记录见 [开发记录.md](开发记录.md)。
