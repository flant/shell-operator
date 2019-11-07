#!/bin/bash -e

workspace=$1

generate_binary() {
    cd $workspace
    go test -tags "test e2e integration" -coverpkg=./... -c cmd/shell-operator/main.go -o $workspace/bin/shell-operator-test

    cat <<'EOF' > $project_bin_tests_dir/werf
#!/bin/bash
script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
EOF

    # Project dir is embedded into script
    cat <<EOF >> $project_bin_tests_dir/werf
project_dir=$project_dir
EOF

    cat <<'EOF' >> $project_bin_tests_dir/werf
coverage_dir=${WERF_TEST_COVERAGE_DIR:-$project_dir/tests_coverage}
mkdir -p $coverage_dir
coverage_file_name="$(date +%s.%N | sha256sum | cut -c 1-10)-$(date +%s).out"
coverage_file_path="$coverage_dir/$coverage_file_name"
exec $script_dir/werf.test -test.coverprofile=$coverage_file_path "$@"
EOF
}

mkdir -p $workspace/bin
generate_binary
chmod +x $workspace/bin/shell-operator-test
