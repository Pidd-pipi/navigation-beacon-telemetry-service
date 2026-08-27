# 航标灯遥测遥控服务（navigation-beacon-telemetry-service）

基于 **Go 1.23** 实现的航标灯遥测 **全栈 Web 应用**：Go 后端服务 + `go:embed` 内嵌原生前端，
离线可跑、无外部 CDN / 无外部服务依赖。系统完成航标灯台账、遥测数据采集、灯质异常诊断、
遥控指令下发与巡检工单闭环。

---

## 一、架构与分层

项目采用清晰的领域驱动分层，`main.go` 仅做装配，业务逻辑禁止下沉到入口文件。

```
HTTP 请求
   │
   ▼
middleware（ErrorHandler → SecurityHeaders → RequestID → AuditLogger → DriftGuard）
   │
   ▼
httpapi（路由 + 处理器 + 统一响应）
   │
   ▼
service（业务用例编排：采集诊断、任务流转、指令回执、定时扫描、审计）
   │
   ▼
store（内存仓储 + JSON 原子持久化）
   │
   ▼
domain（实体 + 枚举 + 状态机 + 领域错误，不含存储与 HTTP 依赖）
```

| 层 | 目录 | 职责 |
| --- | --- | --- |
| 领域层 | `domain/` | 实体、枚举、状态机迁移表、灯质容差/Haversine 距离、领域错误分类 |
| 仓储层 | `store/` | 内存 map 索引，变更后临时文件 + fsync + rename 原子落盘；读接口返回深拷贝 |
| 服务层 | `service/` | 遥测诊断联动、任务流转、指令回执重发、灭灯超时扫描、审计与总览聚合 |
| HTTP 层 | `httpapi/` | REST 路由、请求解析与输入校验、统一 `{code,message,data}` 响应 |
| 横切层 | `middleware/` | panic 恢复、安全响应头、requestID、结构化访问日志、漂移守卫 |
| 前端 | `web/` | 原生 HTML/CSS/JS，通过 `go:embed` 内嵌，组件与 hooks 复用 |

### 目录结构

```
navigation-beacon-telemetry-service/
├── go.mod                      # module example.com/navigation-beacon-telemetry-service, go 1.23
├── main.go                     # 装配入口：仓储/服务/路由/定时器/全超时 HTTP 服务/优雅关闭
├── Dockerfile                  # 多阶段构建（golang:1.23-alpine → alpine:3.20）
├── .dockerignore
├── Makefile
├── runtime_smoke.json          # 冒烟验证配置（保留契约）
├── config/
│   └── config.go               # 全部业务阈值与运行参数（环境变量覆盖 + Validate()）
├── domain/
│   ├── enums.go                # BeaconType/AbnormalityType/TaskStatus/AckStatus/CommandType 等
│   ├── beacon.go               # 航标灯实体 + 运行状态
│   ├── telemetry.go            # 遥测数据 + 灯质校验
│   ├── abnormality.go          # 灯质异常
│   ├── task.go                 # 处置任务 + 状态机
│   ├── command.go              # 遥控指令 + 回执
│   ├── audit.go                # 审计日志
│   ├── position.go             # 经纬度 + Haversine 距离
│   ├── lamp_pattern.go         # 灯质（闪法）+ 容差校验
│   ├── overview.go             # 总览聚合 DTO
│   ├── clone.go                # 实体深拷贝
│   └── errors.go               # 领域错误分类
├── store/
│   ├── store.go                # 仓储根 + 快照序列化/加载（损坏备份降级）
│   ├── beacon_store.go
│   ├── telemetry_store.go
│   ├── abnormality_store.go
│   ├── task_store.go
│   ├── command_store.go
│   └── audit_store.go
├── service/
│   ├── telemetry_service.go    # 采集 + 灯质校验 + 电压健康 + 漂移检测 + 诊断联动
│   ├── abnormality_service.go
│   ├── task_service.go         # 派发/修复/复测/关闭/超时升级
│   ├── command_service.go      # 下发/回执/自动重发
│   ├── sweeper.go              # 灭灯超时扫描定时任务
│   ├── audit_service.go
│   ├── overview_service.go
│   └── seed.go                 # 演示基线数据
├── httpapi/
│   ├── router.go
│   ├── response.go             # 统一响应 + 严格 JSON 解码 + 分页工具
│   ├── health_handler.go
│   ├── beacon_handler.go
│   ├── telemetry_handler.go
│   ├── abnormality_handler.go
│   ├── task_handler.go
│   ├── command_handler.go
│   ├── overview_handler.go
│   └── audit_handler.go
├── middleware/
│   ├── audit.go                # 结构化访问日志（slog）
│   ├── error_handler.go        # 统一错误/panic 处理
│   ├── request_id.go           # requestID 全链路追踪
│   ├── security.go             # 安全响应头 + API no-store
│   └── drift_guard.go          # 漂移期间禁关灯守卫
└── web/
    ├── index.html
    ├── app.js
    ├── api.js
    ├── style.css
    ├── components/
    │   ├── beacon-card.js
    │   ├── telemetry-chart.js
    │   └── command-panel.js
    └── hooks/
        ├── use-beacons.js
        └── use-tasks.js
```

---

## 二、核心业务规则

| 规则 | 实现 |
| --- | --- |
| 灯质校验 | 设定灯质与实测闪法偏差 **>0.5s** 判定灯质异常（`LAMP_TOLERANCE_SEC`） |
| 灭灯故障 | 灯连续熄灭 **>30 分钟** 判定灭灯故障（`LAMP_OUT_MINUTES`） |
| 电压健康 | 电压低于阈值（默认 10.5V）进入低电预警并建议降低遥测频率（30m），电压恢复（≥11.0V，滞回）后解除 |
| 任务状态机 | `created → assigned → repaired → verified → closed`；灭灯任务 **4 小时** 内必须派发，超时升级为紧急 |
| 遥控回执 | 回执超时 **5 分钟** 自动重发，**最多 2 次**；仍失败标记失败并告警 |
| 漂移守卫 | 实测位置偏离锚位超过半径判定漂移；**漂移期间禁止下发关灯指令**（保障可见性） |

---

## 三、运行与测试

### 本地运行

```bash
# 默认监听 8080
go run .

# 指定端口运行
PORT=19012 go run .

# 构建 / 静态检查 / 测试 / 竞态检测
go build ./...
go vet ./...
gofmt -l .
go test ./...
go test -race ./...
```

首次启动会写入演示基线数据（3 座航标 + 近 90 分钟遥测历史）到
`data/beacon_state.json`；删除该文件即可重置演示数据。

### Docker 部署

```bash
# 构建镜像
docker build -t navigation-beacon-telemetry-service:latest .

# 运行（尊重 PORT 环境变量）
docker run --rm -p 8080:8080 navigation-beacon-telemetry-service:latest

# 自定义端口与持久化目录
docker run --rm -p 19012:8080 \
  -e PORT=8080 \
  -v "$PWD/data:/app/data" \
  navigation-beacon-telemetry-service:latest
```

镜像内置 `/healthz` HEALTHCHECK；非 root 用户运行；`CGO_ENABLED=0` 纯静态构建。

---

## 四、环境变量表

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 监听端口（1-65535） |
| `DATA_FILE` | `data/beacon_state.json` | JSON 持久化文件路径；空串表示不落盘 |
| `LOG_LEVEL` | `info` | 结构化日志级别：`debug` / `info` / `warn` / `error` |
| `SERVER_READ_TIMEOUT` | `10s` | HTTP 读超时（含请求头） |
| `SERVER_WRITE_TIMEOUT` | `15s` | HTTP 写超时 |
| `SERVER_IDLE_TIMEOUT` | `60s` | HTTP 空闲连接超时 |
| `LAMP_TOLERANCE_SEC` | `0.5` | 灯质校验偏差容差（秒） |
| `LAMP_OUT_MINUTES` | `30` | 灯连续熄灭判定分钟数 |
| `LOW_VOLTAGE_THRESHOLD` | `10.5` | 低电压阈值（伏） |
| `RECOVERY_VOLTAGE` | `11.0` | 电压恢复阈值（伏，滞回） |
| `DRIFT_RADIUS_M` | `50` | 默认漂移判定半径（米） |
| `COMMAND_ACK_TIMEOUT` | `5m` | 指令回执超时（Go duration，如 `2s`） |
| `COMMAND_MAX_RETRIES` | `2` | 回执超时自动重发上限 |
| `TASK_ASSIGN_DEADLINE` | `4h` | 灭灯任务派发期限 |
| `SWEEP_INTERVAL` | `30s` | 定时扫描周期 |
| `TASK_ESCALATION_SCAN` | `10m` | 灭灯派发超时升级扫描周期 |
| `TELEMETRY_PERIOD` | `15m` | 遥测上报周期（用于演示基线数据） |
| `OFFLINE_AFTER` | `45m` | 无遥测离线判定时长 |
| `AUDIT_RETENTION` | `2000` | 审计日志保留条数 |

启动时 `config.Validate()` 会拒绝非法端口、非法日志级别、负阈值/超时等错误配置。

---

## 五、REST API 表

统一成功响应：`{"code":0,"message":"ok","data":...,"total":N}`。
列表接口均支持 `limit` / `offset`（默认 `limit=20`、上限 `1000`），返回 `total` 为命中过滤条件的总数。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查（200） |
| GET | `/readyz` | 就绪检查（200） |
| GET | `/api/healthz` | 健康检查（JSON） |
| GET | `/api/overview` | 总览聚合 |
| GET | `/api/beacons` | 航标列表（分页） |
| POST | `/api/beacons` | 新建航标 |
| GET | `/api/beacons/{id}` | 航标详情 |
| POST | `/api/beacons/{id}/telemetry` | 遥测上报（灯质校验 + 电压健康 + 漂移检测） |
| GET | `/api/beacons/{id}/telemetry` | 遥测趋势（分页） |
| GET | `/api/abnormalities` | 异常台账（分页，支持 `beacon_id`/`type`/`status` 过滤） |
| POST | `/api/abnormalities` | 手工登记异常（灭灯自动生成任务） |
| GET | `/api/abnormalities/{id}` | 异常详情 |
| POST | `/api/abnormalities/{id}/resolve` | 解决异常 |
| GET | `/api/tasks` | 任务列表（分页，支持 `beacon_id`/`status`/`level` 过滤） |
| POST | `/api/tasks` | 为异常生成任务 |
| POST | `/api/tasks/{id}/assign` | 派发（`created→assigned`） |
| POST | `/api/tasks/{id}/repair` | 修复（`assigned→repaired`） |
| POST | `/api/tasks/{id}/verify` | 复测并关闭（`repaired→verified→closed`） |
| POST | `/api/tasks/{id}/close` | 关闭 |
| POST | `/api/tasks/{id}/escalate` | 升级为紧急 |
| GET | `/api/commands` | 遥控记录（分页，支持 `beacon_id`/`status`/`type` 过滤） |
| POST | `/api/beacons/{id}/command` | 指令下发（`on`/`off`/`switch_pattern`） |
| POST | `/api/commands/{id}/ack` | 终端回执 |
| GET | `/api/audits` | 审计日志（分页） |

### 输入校验

- 请求体上限 **1 MiB**，超出返回 **413**；尾随 JSON、`NaN`/`Inf`、溢出数值、非法 JSON 均返回 **400**。
- 航标/遥测/灯质/位置字段经 `domain.Validate()` 校验，非法输入返回 **400/404/409**，不 panic。
- 遥测电压必须为大于 0 的有限数值；电流为非负有限数值；灯亮时必须上报 `measured_pattern`。

---

## 六、枚举/常量前后端出现位置

| 枚举 | 后端位置 | 前端位置 |
| --- | --- | --- |
| BeaconType：`lighthouse / buoy / daybeacon` | `domain/enums.go` | `web/components/beacon-card.js`、`web/app.js` |
| AbnormalityType：`lamp_mismatch / lamp_out / low_voltage / drift` | `domain/enums.go`、`service/telemetry_service.go`、`service/abnormality_service.go` | `web/app.js`、`web/components/beacon-card.js` |
| TaskStatus：`created / assigned / repaired / verified / closed` | `domain/enums.go`（`TaskTransitions`）、`domain/task.go`、`service/task_service.go` | `web/app.js` |
| AckStatus：`pending / success / failed` | `domain/enums.go`、`domain/command.go`、`service/command_service.go` | `web/app.js` |
| CommandType：`on / off / switch_pattern` | `domain/enums.go`、`domain/command.go`、`middleware/drift_guard.go` | `web/components/command-panel.js` |
| LampState：`on / off` | `domain/enums.go`、`domain/telemetry.go` | `web/app.js`、`web/components/telemetry-chart.js` |
| TaskLevel：`normal / urgent` | `domain/enums.go`、`service/task_service.go` | `web/app.js` |

---

## 七、健康检查与故障排查

| 端点 | 用途 |
| --- | --- |
| `GET /healthz` | 存活/冒烟检查，返回 `200`（`runtime_smoke.json` 的 `ready_url`） |
| `GET /readyz` | 就绪检查，返回 `200` |

### 常见问题

| 现象 | 排查 |
| --- | --- |
| 启动报 `配置校验失败` | 检查 `PORT`、`LOG_LEVEL`、阈值/超时环境变量是否合法 |
| 持久化文件损坏无法启动 | 服务会备份为 `data/beacon_state.json.bak`，记录 warning 后降级为空库启动 |
| 漂移期间关灯返回 409 | 业务守卫：漂移时禁止关灯以保障可见性，回位后重试 |
| 容器健康检查失败 | 确认容器内 `PORT` 与映射端口一致，`wget` 可访问 `127.0.0.1:${PORT}/healthz` |
| 修改了数据仍看到旧值 | 确认 `DATA_FILE` 指向预期目录；删除文件可重置演示数据 |

---

## 八、质量红线与交付说明

- 分层架构：`domain → store → service → httpapi → middleware`，跨文件联动真实存在。
- 状态机迁移表、灯质容差、电压滞回、漂移守卫、指令重发、任务升级全部真实实现，非空壳 CRUD。
- `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./...`、`go test -race ./...` 全绿。
- 仓储读接口返回深拷贝，杜绝调用方原地修改污染内部状态；`-race` 全绿。
- 交付物含 `Dockerfile`、`.dockerignore`、`Makefile`，保留 `runtime_smoke.json` 冒烟契约。
