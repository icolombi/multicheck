FROM golang:1.25.5 AS builder

WORKDIR /app
COPY . .
#RUN go mod init multicheck
RUN go mod tidy

RUN go get ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/multicheck
#RUN go build -o ./bin/multicheck
RUN	strip ./bin/multicheck

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/bin/multicheck .
COPY --from=builder /app/config.toml .

CMD ["./multicheck"]

EXPOSE 8080