FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./cmd/api

FROM alpine:3.20
RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY migrations /app/migrations
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
