# --- Stage 1: Build base & Install Delve ---
FROM golang:1.26.5-alpine AS dev-base
RUN apk add --no-cache git gcc musl-dev
RUN go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /src
COPY buildingblocks ./buildingblocks
COPY trapigo/go.mod trapigo/go.sum ./trapigo/
WORKDIR /src/trapigo
ENV GOWORK=off
RUN go mod download
COPY trapigo/. .

# --- Stage 2: Development / Debugging Target ---
FROM dev-base AS dev
# Expose application port and Delve debugger port
EXPOSE 8080 40000
# Compile on the fly with optimizations disabled and run Delve
CMD ["dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "debug", "./cmd/gateway", "--build-flags=-buildvcs=false", "--", "--config=config.yaml"]

# --- Stage 3: Actual Production Build ---
FROM dev-base AS builder
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o main ./cmd/gateway

FROM alpine:latest AS prod
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /src/trapigo/main .
EXPOSE 8080
CMD ["./main"]