#---------------------build frontend------------------------
FROM docker.io/library/node:22-alpine AS build_frontend
WORKDIR /app/frontend
RUN corepack enable
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm run build

# ---------------------build go--------------------
FROM golang:1.26.4-trixie AS builder_go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=build_frontend /app/frontend/dist /app/frontend/dist

# 安装 ent 与 swagger（swag）
RUN go install entgo.io/ent/cmd/ent@latest
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN go generate ./...

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=rolling" -o shadmin-runner

#-----------------upx shadmin-runner -----------------
FROM backplane/upx:latest AS compressor
WORKDIR /app
COPY --from=builder_go /app/.env.example .
COPY --from=builder_go /app/shadmin-runner .
RUN upx --best --lzma shadmin-runner

#-------------------runner on debian (sqlite)--------------------------
FROM debian:trixie-slim AS runner
WORKDIR /app
COPY --from=compressor /app/shadmin-runner .
COPY --from=builder_go /app/.env.example .
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*
RUN useradd -U -u 1000 appuser && chown -R 1000:1000 /app
USER 1000
EXPOSE 55667
CMD ["./shadmin-runner"]
