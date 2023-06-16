FROM ghcr.io/flant/shell-operator:latest
RUN apk --no-cache add python3
RUN python3 -m ensurepip
RUN pip3 install --no-cache --upgrade pip kubernetes
ADD hooks /hooks
