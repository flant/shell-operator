FROM flant/shell-operator:latest
LABEL app="shell-operator" example="204-validating-webhook"

ADD frameworks/shell /frameworks/shell
# Add hooks
ADD hooks /hooks

# Compiled version of shell-operator binary
ADD shell-operator /
