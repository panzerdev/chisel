# Build stage
FROM golang:alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

# Leverage Docker layer caching for Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary with stripped debug symbols
ARG VERSION
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X github.com/jpillora/chisel/share.BuildVersion=${VERSION:-dev} \
      -X github.com/panzerdev/chisel/share.BuildVersion=${VERSION:-dev} \
      -X main.chiselRepo=https://github.com/panzerdev/chisel" \
    -o /bin/chisel .

# Final minimal Alpine runtime image
FROM alpine:3.21

# Install runtime dependencies: root CA certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user and group
RUN addgroup -S chisel && adduser -S -G chisel -h /app chisel

# Copy binary from builder
COPY --from=builder /bin/chisel /usr/local/bin/chisel

# Create compatibility symlink for legacy /app/bin references
RUN ln -s /usr/local/bin/chisel /app/bin

WORKDIR /app
USER chisel

ENTRYPOINT ["chisel"]
CMD ["--help"]
