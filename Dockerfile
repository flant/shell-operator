FROM golang:1.12-alpine3.9
ARG appVersion=latest
RUN apk --no-cache add git ca-certificates

# Cache-friendly download of go dependencies.
ADD go.mod go.sum /src/shell-operator/
WORKDIR /src/shell-operator
RUN go mod download

ADD . /src/shell-operator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=$appVersion'" -o shell-operator ./cmd/shell-operator

FROM ubuntu:18.04
RUN apt-get update && \
    apt-get install -y ca-certificates wget jq && \
    rm -rf /var/lib/apt/lists && \
    wget https://storage.googleapis.com/kubernetes-release/release/v1.13.5/bin/linux/amd64/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl && \
    mkdir /hooks
COPY --from=0 /src/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_WORKING_DIR /hooks
ENV LOG_TYPE json
ENTRYPOINT ["/shell-operator"]
CMD ["start"]
