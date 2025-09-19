# Prebuilt libjq.
FROM --platform=${TARGETPLATFORM:-linux/amd64} flant/jq:b6be13d5-musl as libjq

# Go builder.
FROM --platform=${TARGETPLATFORM:-linux/amd64} golang:1.23-alpine3.21 AS builder

ARG appVersion=latest
RUN apk --no-cache add git ca-certificates gcc musl-dev libc-dev binutils-gold

# Cache-friendly download of go dependencies.
ADD go.mod go.sum /app/
WORKDIR /app
RUN go mod download

ADD . /app

RUN GOOS=linux \
    go build -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=$appVersion'" \
    -o shell-operator \
    ./cmd/shell-operator

# Final image
FROM --platform=${TARGETPLATFORM:-linux/amd64} alpine:3.21
ARG TARGETPLATFORM
RUN apk --no-cache add ca-certificates bash sed tini curl && \
    kubectlArch=$(echo ${TARGETPLATFORM:-linux/amd64} | sed 's/\/v7//') && \
    echo "Download kubectl for ${kubectlArch}" && \
    wget https://dl.k8s.io/release/v1.30.12/bin/${kubectlArch}/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl && \
    mkdir /hooks
ADD frameworks/shell /frameworks/shell
ADD shell_lib.sh /
COPY --from=libjq /bin/jq /usr/bin
COPY --from=builder /app/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_HOOKS_DIR=/hooks
ENV LOG_TYPE=json
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
