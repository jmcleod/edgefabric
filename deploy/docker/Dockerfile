# SPA build stage
FROM node:20-alpine AS spa-build
WORKDIR /spa
COPY web/console/package*.json ./
RUN npm ci
COPY web/console/ ./
RUN npm run build

# Go build stage
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=spa-build /spa/../static ./web/static/
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-s -w" \
    -o /bin/edgefabric \
    ./cmd/edgefabric

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wireguard-tools && \
    addgroup -S edgefabric && \
    adduser -S -G edgefabric edgefabric && \
    mkdir -p /var/lib/edgefabric /etc/edgefabric && \
    chown -R edgefabric:edgefabric /var/lib/edgefabric

COPY --from=builder /bin/edgefabric /usr/local/bin/edgefabric

EXPOSE 8443 51820/udp 53/udp 53/tcp 80 443

ENTRYPOINT ["edgefabric"]
CMD ["controller", "--config", "/etc/edgefabric/edgefabric.yaml"]
