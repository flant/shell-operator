#!/bin/sh
set -eu

platform=$1
case "$platform" in
    linux/amd64)
        apk_arch=x86_64
        elf_pattern='^ELF 64-bit LSB .*, x86-64,'
        ;;
    linux/arm64)
        apk_arch=aarch64
        elf_pattern='^ELF 64-bit LSB .*, ARM aarch64,'
        ;;
    linux/arm/v7)
        apk_arch=armv7
        elf_pattern='^ELF 32-bit LSB .*, ARM, EABI5'
        ;;
    *)
        echo "Unsupported platform: $platform" >&2
        exit 1
        ;;
esac

actual_arch=$(apk --print-arch)
if [ "$actual_arch" != "$apk_arch" ]; then
    echo "Expected Alpine architecture $apk_arch for $platform, got $actual_arch" >&2
    exit 1
fi

# Emulation can run a binary for the wrong platform, so inspect its ELF header.
for binary in /shell-operator /usr/bin/jq /bin/busybox /sbin/tini /bin/kubectl; do
    description=$(file -bL "$binary")
    echo "$binary: $description"
    if ! printf '%s\n' "$description" | grep -Eq "$elf_pattern"; then
        echo "Unexpected binary architecture for $platform: $binary" >&2
        exit 1
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
