# ---- 构建阶段 ----
FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -ldflags="-s -w" \
    -o /out/server .

# ---- 运行阶段 ----
FROM alpine:3.20

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
RUN mkdir -p /app/data && chown -R app:app /app

COPY --from=build /out/server /app/server

ENV PORT=8080 \
    DATA_FILE=/app/data/beacon_state.json
EXPOSE 8080
USER app

# busybox wget 自带，满足健康检查需求
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/healthz" || exit 1

ENTRYPOINT ["/app/server"]
