FROM golang:1.11-alpine3.9
RUN apk --no-cache add git ca-certificates
ADD . /go/src/github.com/flant/shell-operator
RUN go get -d github.com/flant/shell-operator/...
WORKDIR /go/src/github.com/flant/shell-operator
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o shell-operator ./cmd/shell-operator

FROM alpine:3.9
RUN apk --no-cache add ca-certificates jq &&
    wget https://storage.googleapis.com/kubernetes-release/release/v1.10.12/bin/linux/amd64/kubectl -O /bin/kubectl &&
    chmod +x /bin/kubectl &&
    mkdir /hooks
COPY --from=0 /go/src/github.com/flant/shell-operator/shell-operator /
WORKDIR /
ENV SHELL_OPERATOR_WORKING_DIR /hooks
ENTRYPOINT "/shell-operator"
CMD ["start"]
