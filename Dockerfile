# syntax=docker/dockerfile:1

# Build the frontend first so the binary can embed it.
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# Build the panel.
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/ryxoui ./cmd/ryxoui

# Runtime image.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 ryxoui
COPY --from=build /out/ryxoui /usr/local/bin/ryxoui
USER ryxoui
WORKDIR /var/lib/ryxoui
EXPOSE 8080
ENV RYXOUI_LISTEN=0.0.0.0:8080
VOLUME ["/var/lib/ryxoui"]
ENTRYPOINT ["ryxoui"]
CMD ["serve", "--listen", "0.0.0.0:8080", "--db", "/var/lib/ryxoui/ryxoui.db"]
