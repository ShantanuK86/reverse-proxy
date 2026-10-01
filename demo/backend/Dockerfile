FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init demo-backend && go build -o backend .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/backend /usr/local/bin/backend
ENTRYPOINT ["backend"]
