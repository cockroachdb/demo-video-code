# Compile
FROM golang:1.23.4-alpine AS build
WORKDIR /src
COPY ./*.go /src
COPY go.* /src

RUN CGO_ENABLED=0 GOOS=linux go build -o /app workload.go

# Build
FROM alpine
COPY --from=build app .
ADD https://github.com/golang/go/raw/master/lib/time/zoneinfo.zip /zoneinfo.zip
ENV ZONEINFO /zoneinfo.zip
CMD ./app
