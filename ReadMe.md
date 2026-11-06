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

- 显示东、南、西、北席，东西与南北分别为一队。牌桌使用我、上家、下家、对家。
- 支持定主、锁主、按序攻改、硬主、对子、拖拉机、甩牌、跟牌校验、记牌、抠底、升级与升 A 获胜。
- 同队两人确认弃局后，按当时已结算的打家得分立即结算，不额外抠底。第一人可在第二人确认前撤回。
- 昵称与身份分离。服务端签发会话凭据，浏览器标签页使用 sessionStorage 保存；刷新和网络重连可恢复原座位。新标签页可作为另一位玩家。
- 请求包含 `type`、`payload`、`reqId`；响应 `ack` 包含 `reqId`、`ok`、`code`、`message`、`version`。每个玩家每房间保留最近 128 条回执，重复请求返回原回执与最新快照。
- 客户端一次只提交一个待确认请求；自动重连时使用原编号重试，避免重复执行。
- 有限消息大小、请求频率、每房间连接数、发送背压处理及同源检查。长期无人连接的房间会从内存回收。

## 配置与保存

| 环境变量 | 默认值 | 用途 |
| --- | --- | --- |
| `ADDR` | `:8080` | 服务监听地址 |
| `WEB_DIR` | `./frontend/dist` | 静态前端目录 |
| `DATA_DIR` | `./data` | 牌局、回执、会话签名密钥和诊断日志 |
| `SESSION_SECRET` | 自动保存的随机密钥 | 可选，至少 32 字符；固定后可跨重启验证会话 |
| `ALLOWED_ORIGINS` | 仅同源 | 额外允许的完整 Origin，以逗号分隔 |
| `VITE_WS_URL` | 当前站点 `/ws` | 构建时可指定独立 WebSocket 地址 |

牌局使用临时文件写入、同步后原子替换；保存失败的请求不会提交到房间状态。服务重启后按房间码加载保存状态，所有玩家初始离线并取消准备，重新连接恢复。

`data` 含完整手牌、底牌、身份签名密钥和操作日志，属于服务器私有数据，不对外提供静态访问，也不提交到 Git。请使用同一个数据目录与密钥重启服务。当前文件保存面向单进程部署，不支持多个服务实例同时写同一目录。诊断日志尚未实现完整可视化复盘及保留周期管理。

公网部署需要另行配置 HTTPS/WSS 和反向代理。当前交付为 Web 版；微信登录、微信端客户端及部署联调属于后续迁移工作。

## 项目结构

- `cmd/server`：HTTP 与 WebSocket 服务入口。
- `internal/ws`：签名身份、连接、读写循环、连接接管。
- `internal/room`：串行房间事件循环、协议解析、回执去重、本地保存。
- `internal/game`：阶段状态机、命令边界校验、弃局、结算、私有快照。
- `internal/game/rules`：牌型、比较、跟牌、洗牌与分值。
- `frontend/src`：Vue 界面、强类型协议、连接状态和交互。

规则见 [简版规则.md](简版规则.md)，状态流转见 [阶段模型.md](阶段模型.md)，本轮实施与验证记录见 [开发记录.md](开发记录.md)。
