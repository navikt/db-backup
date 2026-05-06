FROM golang:1.26.2-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /db-backup ./cmd/db-backup

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /db-backup /db-backup
ENTRYPOINT ["/db-backup"]
