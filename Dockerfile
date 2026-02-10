FROM golang:1.20-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o ssd ./

FROM alpine:3.18

RUN addgroup -S ssd && adduser -S ssd -G ssd

RUN mkdir -p /data/ssd /var/log/ssd /etc/ssd \
    && chown -R ssd:ssd /data/ssd /var/log/ssd /etc/ssd

COPY --from=builder /build/ssd /usr/local/bin/ssd
COPY configs/config-docker.yml /etc/ssd/ssd.yml

USER ssd

EXPOSE 8090

VOLUME ["/data/ssd", "/var/log/ssd"]

STOPSIGNAL SIGTERM

ENTRYPOINT ["ssd"]
CMD ["-config", "/etc/ssd/ssd.yml"]
