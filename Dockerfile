# ---------- 1. 前端构建 ----------
FROM node:22-alpine AS web

WORKDIR /app
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund || npm install --no-audit --no-fund
COPY web/ ./
# vite 配置里的 outDir 是 ../internal/web/dist，相对路径在容器里会跑出 /app；
# 用 CLI 参数指到固定位置，不依赖配置文件
RUN npx vite build --outDir /out/dist --emptyOutDir

# ---------- 2. 后端编译（前端产物 embed 进去） ----------
FROM golang:1.26-alpine AS build

RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /out/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/wavenav ./cmd/server

# ---------- 3. 运行 ----------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 wavenav
WORKDIR /app
COPY --from=build /out/wavenav /app/wavenav

ENV WAVENAV_PORT=8018 \
    WAVENAV_DATA=/data \
    WAVENAV_MODE=prod \
    TZ=Asia/Shanghai

RUN mkdir -p /data && chown -R wavenav:wavenav /data /app
USER wavenav

VOLUME ["/data"]
EXPOSE 8018

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8018/api/v1/health >/dev/null || exit 1

CMD ["/app/wavenav"]
