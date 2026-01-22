FROM golang:1.25 AS builder

WORKDIR /app
COPY . .
RUN go mod tidy

RUN go get ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/multicheck
RUN	strip ./bin/multicheck

FROM alpine:latest

# Crea utente non privilegiato
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

WORKDIR /app

# Copia file come root (owner: root:root)
COPY --from=builder --chown=root:root /app/bin/multicheck .
COPY --from=builder --chown=root:root /app/config.toml .

# Rendi l'eseguibile executable ma non writable
RUN chmod 755 /app/multicheck && \
    chmod 644 /app/config.toml

# Passa all'utente non privilegiato
USER appuser

CMD ["./multicheck"]

EXPOSE 8080