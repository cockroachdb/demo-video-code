# Compile
FROM golang:1.20-alpine3.18 AS build
WORKDIR /src
COPY ./*.go /src
COPY go.* /src

RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./main.go

# Build
FROM alpine:3.18
COPY --from=build app .
CMD ./app