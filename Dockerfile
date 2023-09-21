# Prebuilt libjq.
FROM --platform=${TARGETPLATFORM:-linux/amd64} flant/jq:b6be13d5-musl as libjq

# Go builder.
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.20-alpine3.16 AS builder

ARG appVersion=latest
RUN apk --no-cache add git ca-certificates gcc musl-dev libc-dev

# Cache-friendly download of go dependencies.
ADD go.mod go.sum /app/
WORKDIR /app
RUN go mod download

COPY --from=libjq /libjq /libjq
ADD . /app

RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/libjq/include" \
    CGO_LDFLAGS="-L/libjq/lib" \
    GOOS=linux \
    go build -ldflags="-linkmode external -extldflags '-static' -s -w -X 'github.com/flant/shell-operator/pkg/app.Version=$appVersion'" \
            -tags use_libjq \
            -o shell-operator \
            ./cmd/shell-operator

# Final image
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.18
ARG TARGETPLATFORM
RUN mkdir /hooks
ADD frameworks/shell /frameworks/shell
ADD shell_lib.sh /
COPY --from=libjq /bin/jq /usr/bin
COPY --from=builder /app/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_HOOKS_DIR /hooks
ENV LOG_TYPE json
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
