#!/bin/bash

# Скрипт для создания и удаления большого количества подов для тестирования

# Неймспейс, в котором будут создаваться поды
NAMESPACE="pod-cannon"
# Префикс для имен подов
POD_PREFIX="cannon-pod"
# Количество подов для создания
POD_COUNT=1000

# Функция для создания подов
create_pods() {
  echo "Создание ${POD_COUNT} подов с префиксом '${POD_PREFIX}' в неймспейсе '${NAMESPACE}'..."
  for i in $(seq 1 ${POD_COUNT}); do
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_PREFIX}-${i}
  namespace: ${NAMESPACE}
  labels:
    pod-cannon: "true"
spec:
  containers:
  - name: nginx
    image: nginx:1.21.6
EOF
  done
  echo "Все поды отправлены на создание."
}

# Функция для удаления подов
delete_pods() {
  echo "Удаление подов с префиксом '${POD_PREFIX}' в неймспейсе '${NAMESPACE}'..."
  kubectl delete pod -n ${NAMESPACE} -l pod-cannon="true" --force --grace-period=0
  echo "Команда на удаление подов отправлена."
}

# Основная логика скрипта
case "$1" in
  create)
    create_pods
    ;;
  delete)
    delete_pods
    ;;
  *)
    echo "Использование: $0 [create|delete]"
    exit 1
    ;;
esac
