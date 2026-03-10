# VPS container for EdgeFabric enrollment demo.
#
# Alpine with OpenSSH and the edgefabric binary pre-installed.
# SSH in with: ssh root@localhost -p 2222 (password: demo)
#
# Multi-stage: copies edgefabric binary from the main build image.

FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags "-s -w" \
    -o /bin/edgefabric \
    ./cmd/edgefabric

# Runtime
FROM alpine:3.21

RUN apk add --no-cache openssh-server && \
    ssh-keygen -A && \
    echo "root:demo" | chpasswd && \
    sed -i 's/#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    mkdir -p /var/lib/edgefabric /etc/edgefabric

COPY --from=builder /bin/edgefabric /usr/local/bin/edgefabric

EXPOSE 22

CMD ["/usr/sbin/sshd", "-D", "-e"]
