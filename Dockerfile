# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /nexus ./cmd/nexus
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /nexus-mcp ./cmd/nexus-mcp

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -g '' nexus

WORKDIR /app

COPY --from=builder /nexus .
COPY --from=builder /nexus-mcp .

USER nexus

EXPOSE 8080 8081

ENTRYPOINT ["./nexus"]
