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
    git submodule update --init && \
    /libjq-go/scripts/install-libjq-dependencies-ubuntu.sh && \
    /libjq-go/scripts/build-libjq-static.sh /libjq-go /out


# build shell-operator binary
FROM golang:1.12 AS shell-operator
ARG appVersion=latest

# Cache-friendly download of go dependencies.
ADD go.mod go.sum /src/shell-operator/
WORKDIR /src/shell-operator
RUN go mod download

COPY --from=libjq /out/build /build
ADD . /src/shell-operator

RUN CGO_ENABLED=1 \
    CGO_CFLAGS="-I/build/jq/include" \
    CGO_LDFLAGS="-L/build/onig/lib -L/build/jq/lib" \
    GOOS=linux \
    go build -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=$appVersion'" \
             -o shell-operator \
             ./cmd/shell-operator


# build final image
FROM ubuntu:18.04
RUN apt-get update && \
    apt-get install -y ca-certificates wget jq && \
    rm -rf /var/lib/apt/lists && \
    wget https://storage.googleapis.com/kubernetes-release/release/v1.13.5/bin/linux/amd64/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl && \
    mkdir /hooks
ADD frameworks /
COPY --from=shell-operator /src/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_WORKING_DIR /hooks
ENV LOG_TYPE json
ENTRYPOINT ["/shell-operator"]
CMD ["start"]
