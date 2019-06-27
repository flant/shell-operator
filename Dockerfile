FROM golang:1.12-alpine3.9
ARG appVersion=latest
RUN apk --no-cache add git ca-certificates
ADD . /go/src/github.com/flant/shell-operator
RUN go get -d github.com/flant/shell-operator/...
WORKDIR /go/src/github.com/flant/shell-operator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'github.com/flant/shell-operator/pkg/app.Version=$appVersion'" -o shell-operator ./cmd/shell-operator

FROM ubuntu:18.04
RUN apt-get update && \
    apt-get install -y ca-certificates wget jq && \
    rm -rf /var/lib/apt/lists && \
    wget https://storage.googleapis.com/kubernetes-release/release/v1.13.5/bin/linux/amd64/kubectl -O /bin/kubectl && \
    chmod +x /bin/kubectl && \
    mkdir /hooks
COPY --from=0 /go/src/github.com/flant/shell-operator/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_WORKING_DIR /hooks
ENTRYPOINT ["/shell-operator"]
CMD ["start"]
