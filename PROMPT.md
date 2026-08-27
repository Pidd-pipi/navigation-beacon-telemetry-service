# 航标灯遥测遥控服务（navigation-beacon-telemetry-service）

## 一、项目概述

基于 Go 实现的航标灯遥测 Web 项目，一款后端服务，完成航标灯台账、遥测数据采集、灯质异常诊断、遥控指令下发与巡检工单闭环。

项目类型：**全栈 Web 应用**（Go 后端服务 + `go:embed` 内嵌前端页面）。

## 二、业务背景与领域规则

海上航标灯（灯塔、浮标、导标）保障船舶航行安全，需要实时掌握其工作状态：灯光是否正常发光、灯质（闪法）是否符合设定、电池电压是否健康、是否漂移。系统通过遥测终端定期回传状态数据，异常时诊断并生成处置任务，也可远程遥控（开灯/关灯/切换灯质）。每次遥控和处置都要留痕。

关键领域规则（这些规则是后续埋 bug 验证跨文件改动的核心约束，必须真实实现）：

1. 遥测数据：终端按 15 分钟周期上报（beacon_id + 灯状态 + 电压 + 电流 + 位置 + 灯质），服务端按灯质设定校验实际发光闪法是否匹配。
2. 灯质校验：设定灯质（如闪 2s/灭 2s）与上报的实测闪法偏差超 0.5s 判定灯质异常；灯连续熄灭超 30 分钟判定灭灯故障。
3. 电压健康：电池电压低于下限阈值判定低电压，进入「低电预警」并降低遥测频率建议；电压恢复后才解除。
4. 处置任务状态机：已生成(created) → 已派发(assigned) → 已修复(repaired) → 已复测(verified) → 已关闭(closed)；灭灯故障任务 4 小时内必须派发，超时升级。
5. 遥控指令：开灯/关灯/切换灯质指令下发后需收到终端回执（成功/失败）；回执超时 5 分钟自动重发最多 2 次，仍失败标记指令失败并告警。
6. 漂移检测：上报位置与锚位距离超过设定半径判定漂移，漂移期间禁止下发关灯指令（保障可见性）。

## 三、核心实体（≥3 个，必须贯穿全栈）

每个实体必须贯穿「数据库/存储表 → domain model → repository → service → handler → 前端 API 层 → 前端页面/组件」全链路。

| 实体 | 关键字段 | 业务动作 |
|---|---|---|
| 航标灯 Beacon | id、类型(灯塔/浮标/导标)、锚位、设定灯质、状态、低电标记 | 台账、状态查询 |
| 遥测数据 TelemetryData | id、航标id、灯状态、电压、电流、位置、实测灯质、时间戳 | 采集、校验 |
| 灯质异常 LampAbnormality | id、航标id、类型(灯质偏差/灭灯)、状态、首次发现时间 | 诊断 |
| 处置任务 DisposalTask | id、异常id、级别、派发人、状态、期限、复测结果 | 派发、修复、复测 |
| 遥控指令 RemoteCommand | id、航标id、指令类型、下发时间、回执状态、重试次数 | 下发、回执 |

## 四、核心页面与 API

### 前端页面（≥4 个路由，至少 2 个页面共用同一个业务组件）

| 项目 | 说明 |
|---|---|
| / 航标总览 | 航标状态卡片 + 灭灯/低电/漂移标记 | Beacon |
| /beacons/{id} 航标详情 | 遥测趋势 + 灯质校验 + 遥控面板 | Beacon、TelemetryData |
| /abnormalities 异常台账 | 灯质异常列表 + 诊断 | LampAbnormality |
| /tasks 处置任务 | 任务列表 + 派发/修复/复测/关闭 | DisposalTask |
| /commands 遥控记录 | 指令下发与回执历史 | RemoteCommand |

### 后端 REST API（与页面一一对应，命中真实业务链路）

| 项目 | 说明 |
|---|---|
| POST /api/beacons/{id}/telemetry | 遥测上报（灯质校验 + 电压健康 + 漂移检测） |
| POST /api/abnormalities | 生成异常（或由上报自动生成） |
| POST /api/tasks | 生成处置任务 |
| POST /api/tasks/{id}/assign | 派发任务 |
| POST /api/tasks/{id}/verify | 复测并关闭 |
| POST /api/beacons/{id}/command | 遥控指令下发 |
| POST /api/commands/{id}/ack | 终端回执 |
| GET /api/beacons/{id}/telemetry | 遥测趋势 |
| GET /api/overview | 总览聚合 |
| GET /api/healthz | 健康检查 |

## 五、横切关注点（≥2 个）

1. 操作审计日志：遥控下发、任务流转全部留痕；触达 handler → service → audit store。
2. 灭灯超时扫描定时任务：每 10 分钟扫描灭灯任务派发超时并升级；触达 service → store → 任务页。
3. 全局错误处理与统一响应格式。

## 六、共享枚举/常量（≥2 组）

枚举/常量要求前后端各自定义且保持一致，README 中列出所有出现位置。

1. 航标类型 BeaconType：lighthouse / buoy / daybeacon。
2. 异常类型 AbnormalityType：lamp_mismatch / lamp_out / low_voltage / drift。
3. 任务状态 TaskStatus：created / assigned / repaired / verified / closed；指令回执 AckStatus：pending / success / failed。

## 七、共享前端组件与 hooks（组件 ≥3 个、hooks ≥2 个）

### 共享组件（放 `web/components/`）

1. BeaconCard：航标状态卡片，被总览与详情共用。
2. TelemetryChart：遥测趋势图，被详情与异常页共用。
3. CommandPanel：遥控指令面板，被详情与遥控记录共用。

### 自定义 hooks（放 `web/hooks/`）

1. useBeacons(poll)：航标列表轮询，被总览与详情共用。
2. useTasks(filter)：处置任务，被任务页与总览共用。

## 八、后端中间件（≥2 个）

1. auditLogger：审计日志中间件。
2. errorHandler：统一错误/panic 处理中间件。
3. driftGuard：漂移期间禁关灯守卫中间件。

## 九、技术要求

- 语言：**Go 1.23**（go.mod 声明 `go 1.23`，module 路径 `example.com/navigation-beacon-telemetry-service`）
- 运行：`go run .` 默认监听 `8080`，支持 `PORT` 环境变量覆盖
- 存储：SQLite（`modernc.org/sqlite` 纯 Go 驱动，CGO 关闭）或内置内存仓储 + JSON 文件持久化，二选一，必须可重复构建、无外部服务依赖
- 前端：纯原生 HTML/CSS/JS，`go:embed` 内嵌 `web/` 静态资源，禁止引入外部 CDN 依赖（离线可跑）
- 服务入口：`GET /healthz` 返回 200；页面 `GET /` 可访问
- 根目录必须包含 `runtime_smoke.json`：`mode: service` + `start: go run .` + `ready_url: /healthz`；`project_intro` 一句话简介必须包含项目类型（如「基于 Go 实现的XXX Web 项目，一款后端服务，完成……」）
- 根目录必须包含 `README.md`：项目说明、目录结构、运行与测试命令、环境变量说明
- 构建：`go build ./...` 与 `go test ./...` 必须全部通过（基线干净、无 bug）

## 十、文件结构强制清单（规模目标：≥2000 行 Go 功能代码、≥20 个 `.go` 文件）

```
backend/
├── go.mod
├── main.go
├── config/
│   └── config.go            # 电压阈值、灯质容差、超时重发参数
├── domain/
│   ├── beacon.go            # 航标灯实体 + 状态
│   ├── telemetry.go         # 遥测数据 + 灯质校验
│   ├── abnormality.go       # 异常诊断
│   ├── task.go              # 处置任务状态机
│   └── command.go           # 遥控指令 + 回执
├── store/
│   ├── beacon_store.go
│   ├── telemetry_store.go
│   ├── abnormality_store.go
│   ├── task_store.go
│   ├── command_store.go
│   └── audit_store.go
├── service/
│   ├── telemetry_service.go # 采集 + 校验 + 诊断
│   ├── abnormality_service.go
│   ├── task_service.go      # 派发/修复/复测
│   ├── command_service.go   # 下发/回执/重发
│   ├── sweeper.go           # 灭灯超时扫描
│   └── audit_service.go
├── httpapi/
│   ├── router.go
│   ├── beacon_handler.go
│   ├── telemetry_handler.go
│   ├── abnormality_handler.go
│   ├── task_handler.go
│   ├── command_handler.go
│   └── health_handler.go
├── middleware/
│   ├── audit.go
│   ├── error_handler.go
│   └── drift_guard.go
└── web/
    ├── index.html
    ├── app.js
    ├── style.css
    ├── components/
    └── hooks/
```

**严禁合并职责到单一文件**：handler、service、repository、domain 必须分层；禁止把所有逻辑塞进 `main.go` 或一个 `handlers.go`。目标规模下限 2000 行 / 20 个 `.go` 文件，实际建议做到 3000 行以上 / 30 个文件以上，保证每个业务模块（实体、状态机、联动、报表）都有独立文件。

## 十一、运行、测试与交付要求

1. `go build ./...` 通过；`go test ./...` 全绿（含各业务模块的单元测试，测试文件不计入规模）。
2. `go run .` 后 `GET /healthz` 返回 200，前端页面 `GET /` 可打开且核心接口可用。
3. 每个核心业务动作都要有可复现的输入（API 请求/页面操作），方便后续构造缺陷与验证命令。
4. 代码中不得出现任何「故意埋错」「TODO bug」类注释；交付为干净基线。

## 十二、质量红线

1. **天然多文件、多层耦合**：任何一个小改动（如给某状态新增一个合法迁移）都应触达 3-5 个文件（domain + repository + service + handler + 前端组件 + 枚举定义）。
2. 业务规则必须具体、可验证：状态机迁移表、联动逻辑、校验边界、生命周期管理必须真实存在，禁止空壳 CRUD。
3. 本项目用于评测跨文件协同改动能力，禁止做成本目录、对账/财务、库存盘点、电商订单、预约挂号、工单客服、数据可视化报表类业务。
4. 前端页面必须真实消费后端接口，禁止纯静态假页面。

---
*生成说明：本提示词面向 Go 标注数据流水线 2000 行档位，主题已对照禁选题材清单核验。*
