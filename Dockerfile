# Use an official Golang image as a base for the build stage
FROM golang:1.22-alpine as builder

WORKDIR /app

COPY main.go go.sum go.mod .

RUN go build -o gitlab-pusher main.go

FROM alpine:latest

ARG GUID=30000

RUN addgroup -S $GUID  && adduser -D -u $GUID oxidized

USER $GUID

WORKDIR /app

COPY --from=builder /app/gitlab-pusher .

ENTRYPOINT ["./gitlab-pusher"]
