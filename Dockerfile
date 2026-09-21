# Build stage
FROM golang:1.27.1-alpine3.24 AS builder
RUN apk add --no-cache git
WORKDIR /app
ENV CGO_ENABLED=0 GOOS=linux
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build args for version info (CI/release)
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE
RUN BUILD_DATE=${BUILD_DATE:-$(date +%FT%T%z)} && \
    go build -ldflags "-w -s -X 'github.com/soulteary/version-kit/v4.Version=$VERSION' -X 'github.com/soulteary/version-kit/v4.Commit=$COMMIT' -X 'github.com/soulteary/version-kit/v4.BuildDate=$BUILD_DATE'" -o herald-smtp .

# Runtime stage
FROM alpine:3.24
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /app/herald-smtp /bin/herald-smtp
RUN addgroup -g 10001 -S herald && \
    adduser -u 10001 -S -D -G herald -H -s /sbin/nologin herald
USER 10001:10001
EXPOSE 8084
CMD ["herald-smtp"]
