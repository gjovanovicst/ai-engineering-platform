#!/usr/bin/env bash
# Verify the agent sandbox's Docker hardening flags.
set -euo pipefail
IMAGE=${IMAGE:-aip/sandbox:go1.25-node22}

out=$(docker run --rm --cap-drop ALL --security-opt no-new-privileges --read-only \
    --network none --pids-limit 512 \
    --tmpfs /workspace:rw,exec,size=64m,uid=10001,gid=10001 "$IMAGE" sh -c '
touch /root/pwn 2>/dev/null; echo root_write=$?
touch /workspace/ok 2>/dev/null; echo workspace_write=$?
mkdir /etc/pwn 2>/dev/null; echo etc_mkdir=$?
mount -t tmpfs none /mnt 2>/dev/null; echo mount=$?
grep "^CapEff:" /proc/self/status
command -v wget >/dev/null && (wget -qO- -T 2 http://1.1.1.1 >/dev/null 2>&1; echo network=$?) || echo network_tool=absent
')
printf '%s\n' "$out"
for expected in root_write=1 etc_mkdir=1 mount=1; do
    echo "$out" | grep -qx "$expected" || { echo "missing $expected" >&2; exit 1; }
done
echo "$out" | grep -qx 'workspace_write=0' || { echo "workspace not writable" >&2; exit 1; }
echo "$out" | grep -q 'CapEff:.*0000000000000000' || { echo "caps not dropped" >&2; exit 1; }
if echo "$out" | grep -q '^network='; then
    echo "$out" | grep -qx 'network=1' || { echo "network reachable" >&2; exit 1; }
fi
echo "sandbox hardening drill ok"
