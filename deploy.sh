#!/bin/bash
set -e

SSH="/c/Windows/System32/OpenSSH/ssh.exe"
SCP="/c/Windows/System32/OpenSSH/scp.exe"
HOST="root@picokvm.local"
BINARY="bin/kvm_app"
REMOTE_PATH="/userdata/picokvm/bin/kvm_app"

echo "Building..."
make build_dev

echo "Uploading..."
$SCP "$BINARY" "$HOST:/tmp/kvm_app_new"

echo "Installing and rebooting..."
$SSH "$HOST" "cp $REMOTE_PATH ${REMOTE_PATH}.bak && mv /tmp/kvm_app_new $REMOTE_PATH && chmod +x $REMOTE_PATH && reboot"

echo "Done — device rebooting. Wait ~20s for it to come back up."
