# build libjq
FROM ubuntu:18.04 AS libjq
ENV DEBIAN_FRONTEND=noninteractive \
    DEBCONF_NONINTERACTIVE_SEEN=true \
    LC_ALL=C.UTF-8 \
    LANG=C.UTF-8

RUN apt-get update && \
    apt-get install -y git ca-certificates && \
    git clone https://github.com/flant/libjq-go /libjq-go && \
    cd /libjq-go && \
    git submodule update --init --recursive && \
    /libjq-go/scripts/install-libjq-dependencies-ubuntu.sh && \
    /libjq-go/scripts/build-libjq-static.sh /libjq-go /libjq


# build shell-operator binary
FROM golang:1.12 AS shell-operator
ARG appVersion=latest

# Cache-friendly download of go dependencies.
ADD go.mod go.sum /src/shell-operator/
WORKDIR /src/shell-operator
RUN go mod download

COPY --from=libjq /libjq /libjq
ADD . /src/shell-operator

RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/libjq/include" \
    CGO_LDFLAGS="-L/libjq/lib" \
    GOOS=linux \
    go build -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=$appVersion'" \
             -o shell-operator \
             ./cmd/shell-operator

FROM krallin/ubuntu-tini:bionic AS tini

# build final image
FROM ubuntu:18.04
RUN apt-get update && \
    apt-get install -y ca-certificates wget jq && \
    rm -rf /var/lib/apt/lists && \
    wget https://storage.googleapis.com/kubernetes-release/release/v1.17.4/bin/linux/amd64/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl && \
    mkdir /hooks
ADD frameworks /
ADD shell_lib.sh /
COPY --from=tini /usr/local/bin/tini /sbin/tini
COPY --from=shell-operator /src/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_HOOKS_DIR /hooks
ENV LOG_TYPE json
ENTRYPOINT ["/sbin/tini", "--", "/shell-operator"]
CMD ["start"]
