FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o proxy ./cmd/proxy

FROM scratch
COPY --from=builder /app/proxy /usr/local/bin/proxy
COPY --from=builder /app/config/proxy.yaml /etc/proxy/proxy.yaml
COPY --from=builder /app/certs/ /etc/proxy/certs/
ENTRYPOINT ["proxy"]
CMD ["--config", "/etc/proxy/proxy.yaml"]
