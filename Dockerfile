FROM golang:1.26 AS builder

WORKDIR /app

# Copia solo i file delle dipendenze prima per sfruttare la cache
COPY go.mod go.sum ./
RUN go mod download

# Copia il codice sorgente e builda
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./bin/multicheck && \
    strip ./bin/multicheck

FROM alpine:latest

WORKDIR /app

# Copia file dal builder
COPY --from=builder --chown=root:root /app/bin/multicheck .
COPY --chown=root:root config.toml .

# Crea utente, imposta permessi in un singolo layer
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser && \
    chmod 755 /app/multicheck && \
    chmod 644 /app/config.toml

USER appuser

EXPOSE 8080

CMD ["./multicheck"]