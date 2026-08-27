# navigation-beacon-telemetry-service__003

基于 Go 实现的航标灯遥测遥控全栈 Web 项目，一款后端服务，完成航标灯台账、遥测采集、灯质异常诊断、遥控指令下发与巡检工单闭环。
## 构建镜像

请从**仓库根目录**执行；`benzhi.Dockerfile`、`build_benzhi_docker.sh`、`BENZHI_README.md` 均固定在该目录：

```bash
./build_benzhi_docker.sh <image-name> [linux/amd64|linux/arm64]
```

## 标准命令

```bash
go build ./...     # 编译
go run .   # 启动
go test ./...      # 测试（如有）
```

## 环境

- 基础镜像: golang:1.23
- Go 模块目录: `.`
- 依赖已在镜像构建阶段预下载，容器内离线可用。
- 容器内工作目录: `/app`
