FROM golang:1.26.5-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -ldflags="-s -w" -o main ./cmd/app

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /src/main .

EXPOSE 8080

CMD ["./main"]
