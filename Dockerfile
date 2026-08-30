# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/notify ./cmd/notify

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/notify /notify
EXPOSE 8080
ENTRYPOINT ["/notify"]
