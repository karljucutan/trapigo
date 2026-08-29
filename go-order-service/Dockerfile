FROM golang:1.26.5-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git gcc musl-dev

COPY buildingblocks ./buildingblocks
COPY go-order-service/go.mod go-order-service/go.sum ./go-order-service/

WORKDIR /src/go-order-service
ENV GOWORK=off
RUN go mod download
COPY go-order-service/. .
RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o main ./cmd/app

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /src/go-order-service/main .

EXPOSE 8080

CMD ["./main"]
