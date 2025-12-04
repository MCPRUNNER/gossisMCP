# syntax=docker/dockerfile:1.6
ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssis-analyzer .

FROM alpine:3.20
RUN adduser -S -D -H ssis && mkdir -p /app && chown ssis:ssis /app
WORKDIR /app
COPY --from=builder /out/ssis-analyzer /usr/local/bin/ssis-analyzer
COPY config.yaml config.json README.md ./
COPY Documents ./Documents
USER ssis
ENV GOSSIS_PKG_DIRECTORY=/app/Documents/SSIS_EXAMPLES \
    GOSSIS_HTTP_PORT=8086
EXPOSE 8086
ENTRYPOINT ["/usr/local/bin/ssis-analyzer"]
CMD ["--http", "--port", "8086"]
