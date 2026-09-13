FROM golang:1.27-alpine AS builder
WORKDIR /app/
ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o backup-to-nextcloud .

FROM golang:1.27-alpine
WORKDIR /app/
COPY --from=builder /app/backup-to-nextcloud /app/backup-to-nextcloud
ENTRYPOINT ["/app/backup-to-nextcloud"]
