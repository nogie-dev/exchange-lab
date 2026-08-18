FROM golang:1.25.7-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o /out/matching-engine ./cmd/server
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.1

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/matching-engine ./matching-engine
COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY config ./config
COPY db/migrations ./db/migrations

EXPOSE 8080
