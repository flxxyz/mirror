FROM golang:1.24-alpine3.22 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/mirror main.go

FROM alpine:3.22 AS runner
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/mirror /app/mirror
CMD ["/app/mirror"]
