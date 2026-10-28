#!/bin/sh
set -eu

report_architecture_error() {
    cat >&2 <<EOF
❌ Wrong CPU architecture
   ├─ Component: $1
   ├─ Target platform: $platform
   ├─ Expected: $2
   └─ Found: $3

   Build stopped to prevent publishing an incompatible image.
EOF
    exit 1
}

describe_architecture() {
    case "$1" in
        x86_64) echo 'AMD64 (x86_64)' ;;
        aarch64) echo 'ARM64 (aarch64)' ;;
        armv7) echo 'ARM32 (armv7)' ;;
        'ELF 64-bit LSB '*', x86-64,'*) echo 'AMD64 (x86-64)' ;;
        'ELF 64-bit LSB '*', ARM aarch64,'*) echo 'ARM64 (aarch64)' ;;
        'ELF 32-bit LSB '*', ARM, EABI5'*) echo 'ARM32 (ARM EABI5)' ;;
        *) printf '%s\n' "$1" ;;
    esac
}

platform=$1
case "$platform" in
    linux/amd64)
        apk_arch=x86_64
        expected_cpu=AMD64
        elf_pattern='^ELF 64-bit LSB .*, x86-64,'
        ;;
    linux/arm64)
        apk_arch=aarch64
        expected_cpu=ARM64
        elf_pattern='^ELF 64-bit LSB .*, ARM aarch64,'
        ;;
    linux/arm/v7)
        apk_arch=armv7
        expected_cpu=ARM32
        elf_pattern='^ELF 32-bit LSB .*, ARM, EABI5'
        ;;
    *)
        echo "Unsupported platform: $platform" >&2
        exit 1
        ;;
esac

printf '🔍 %s: starting architecture consistency checks for the base image and built binaries.\n' "$platform"

actual_arch=$(apk --print-arch)
if [ "$actual_arch" != "$apk_arch" ]; then
    report_architecture_error "Alpine base image" \
        "$(describe_architecture "$apk_arch")" "$(describe_architecture "$actual_arch")"
fi

# Emulation can run a binary for the wrong platform, so inspect its ELF header.
for binary in /shell-operator /usr/bin/jq /bin/busybox /sbin/tini /bin/kubectl; do
    description=$(file -bL "$binary")
    echo "$binary: $description"
    if ! printf '%s\n' "$description" | grep -Eq "$elf_pattern"; then
        report_architecture_error "$binary" "$expected_cpu" "$(describe_architecture "$description")"
    fi
done

/sbin/tini -- /shell-operator version
/bin/kubectl version --client

context_file=$(mktemp)
trap 'rm -f "$context_file"' EXIT
printf '%s\n' '[{"binding":"image-verification"}]' > "$context_file"
export BINDING_CONTEXT_PATH="$context_file"
export BINDING_CONTEXT_CURRENT_INDEX=0
bash -ec 'source /shell_lib.sh; test "$(context::get --required binding)" = image-verification'

printf '✅ %s: Alpine and all 5 checked binaries have the expected CPU architecture.\n' "$platform"
