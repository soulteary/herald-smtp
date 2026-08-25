# Build stage
FROM golang:1.26.6-alpine3.23 AS builder
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
    go build -ldflags "-w -s -X 'github.com/soulteary/version-kit.Version=$VERSION' -X 'github.com/soulteary/version-kit.Commit=$COMMIT' -X 'github.com/soulteary/version-kit.BuildDate=$BUILD_DATE'" -o herald-smtp .

# Runtime stage
FROM alpine:3.23
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /app/herald-smtp /bin/herald-smtp
RUN addgroup -S herald && adduser -S -G herald herald
USER herald
EXPOSE 8084
CMD ["herald-smtp"]
