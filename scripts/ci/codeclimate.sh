#!/bin/bash -e

curl -L https://codeclimate.com/downloads/test-reporter/test-reporter-latest-linux-amd64 > ./cc-test-reporter
chmod +x ./cc-test-reporter

coverage_files=$(find $COVERAGE_DIR -name '*.out')
for file in ${coverage_files[@]}
do
  file_name=$(echo $file | tr / _)
  ./cc-test-reporter format-coverage \
    -t=gocov \
    -o="cc-coverage/$file_name.codeclimate.json" \
    -p=github.com/flant/shell-operator \
    "$file"
done

./cc-test-reporter sum-coverage \
  -o="cc-coverage/codeclimate.json" \
  -p=$(ls -1q cc-coverage/*.codeclimate.json | wc -l) \
  cc-coverage/*.codeclimate.json


if [[ ${CC_TEST_REPORTER_ID} == "" ]] ; then
  echo "Set \$CC_TEST_REPORTER_ID to upload coverage report!"
  exit 1
fi

./cc-test-reporter upload-coverage \
  -i="cc-coverage/codeclimate.json"
