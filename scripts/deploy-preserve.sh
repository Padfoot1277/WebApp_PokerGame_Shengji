#!/usr/bin/env bash
set -euo pipefail
base=/home/ubuntu/shengji
exec 9>/home/ubuntu/shengji-preserve.lock
flock -n 9 || { echo 'Another preserve deployment is running.'; exit 1; }
test "$(readlink -f "$base/data")" = /home/ubuntu/shengji/data
test ! -L "$base/data"
stage=$(mktemp -d /home/ubuntu/shengji-preserve-stage.XXXXXX)
backup=/home/ubuntu/shengji-backups/$(date +%Y%m%d-%H%M%S)-$$
trap 'rm -rf "$stage"' EXIT
tar -xzf "$1" -C "$stage"
test -s "$stage/shengji-server"
test -s "$stage/dist/index.html"
mkdir -p "$backup"
cp -a "$base/server" "$backup/server"
cp -a "$base/frontend/dist" "$backup/dist"
echo "Backup location: $backup"
stopped=0
switched=0
recover() {
    status=$?
    trap - EXIT
    if [ "$status" -ne 0 ] && [ "$stopped" = 1 ]; then
        echo 'Deployment failed; restoring previous application. Live data is retained.' >&2
        sudo -n systemctl stop shengji || true
        if [ "$switched" = 1 ]; then
            cp "$backup/server" "$base/server.rollback"
            chmod 755 "$base/server.rollback"
            mv "$base/server.rollback" "$base/server"
            cp -a "$backup/dist/." "$base/frontend/dist/"
        fi
        sudo -n systemctl start shengji || true
        echo "Pre-deployment data backup: $backup/data (manual recovery only)" >&2
    fi
    rm -rf "$stage"
    exit "$status"
}
trap recover EXIT
sudo -n systemctl stop shengji
stopped=1
cp -a "$base/data" "$backup/data"
switched=1
install -m 755 "$stage/shengji-server" "$base/server.new"
mv "$base/server.new" "$base/server"
# Keep old hashed assets for browsers that loaded the previous page.
cp -a "$stage/dist/." "$base/frontend/dist/"
sudo -n systemctl start shengji
healthy=0
for attempt in $(seq 1 15); do
    if systemctl is-active --quiet shengji && curl --fail --silent --max-time 2 http://127.0.0.1:8080/healthz; then
        healthy=1
        break
    fi
    sleep 1
done
test "$healthy" = 1
stopped=0
rm -f "$1"
echo
echo 'Deployment complete. Rooms, sessions and logs were preserved.'
