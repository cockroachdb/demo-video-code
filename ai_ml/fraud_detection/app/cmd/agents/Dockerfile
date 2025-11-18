FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/app ./ai_ml/fraud_detection/app/cmd/agents

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/app /usr/local/bin/app

ENTRYPOINT ["/usr/local/bin/app"]