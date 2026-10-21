// Package installer holds the script that hands a machine to the panel.
//
// The script is served by the panel under /install/ryxoui.sh. It runs on a
// fresh machine, registers the machine at the panel and gives the panel the
// key it needs to drive it.
package installer

// Script is the installer of a node.
//
// It is a plain POSIX shell script, so it runs on every machine that has a
// shell and a curl. It never keeps the token it was given: the token is used
// for the two calls and then forgotten.
const Script = `#!/bin/sh
# RyxoUI node installer.
#
# It registers this machine at the panel and gives the panel the key it needs
# to drive it. The token is used for the two calls and is not kept.

set -eu

PANEL=""
NAME=""
ADDRESS=""
TOKEN=""
SSH_PORT="22"
SSH_USER="root"
CA_DIR="/usr/local/share/ryxoui"

usage() {
  cat <<'USAGE'
Usage: ryxoui.sh --panel URL --name NAME --address ADDRESS --token TOKEN [options]

  --panel URL       the address of the panel, for example https://panel.example
  --name NAME       the name of the node in the panel
  --address ADDRESS the address of this machine for the panel
  --token TOKEN     an API token of the panel with the scope nodes:write
  --ssh-port PORT   the SSH port of this machine (default 22)
  --ssh-user USER   the account the panel uses (default root)
  --help            show this text
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --panel) PANEL="${2:-}"; shift 2 ;;
    --name) NAME="${2:-}"; shift 2 ;;
    --address) ADDRESS="${2:-}"; shift 2 ;;
    --token) TOKEN="${2:-}"; shift 2 ;;
    --ssh-port) SSH_PORT="${2:-}"; shift 2 ;;
    --ssh-user) SSH_USER="${2:-}"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

if [ -z "$PANEL" ] || [ -z "$NAME" ] || [ -z "$ADDRESS" ] || [ -z "$TOKEN" ]; then
  usage
  exit 1
fi

if [ "$(id -u)" != "0" ]; then
  echo "the installer needs the rights of root" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is needed to talk to the panel" >&2
  exit 1
fi

echo "registering $NAME at $PANEL"

# The authority of the panel is kept, so the node can trust it later on.
mkdir -p "$CA_DIR"
if ! curl -fsSL "$PANEL/install/ca.pem" -o "$CA_DIR/ca.pem"; then
  echo "the authority of the panel could not be fetched" >&2
  exit 1
fi

# The node registers itself and receives the key of the panel.
response=$(curl -fsSL -X POST "$PANEL/install/register" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$NAME\",\"address\":\"$ADDRESS\",\"sshPort\":$SSH_PORT,\"sshUser\":\"$SSH_USER\"}")

node_id=$(printf '%s' "$response" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
public_key=$(printf '%s' "$response" | sed -n 's/.*"sshPublicKey":"\([^"]*\)".*/\1/p')

if [ -z "$node_id" ] || [ -z "$public_key" ]; then
  echo "the panel did not answer with a node: $response" >&2
  exit 1
fi

# The key of the panel is added to the account it uses.
home=$(getent passwd "$SSH_USER" 2>/dev/null | cut -d: -f6)
if [ -z "$home" ]; then
  home="/root"
fi

mkdir -p "$home/.ssh"
chmod 700 "$home/.ssh"
touch "$home/.ssh/authorized_keys"
chmod 600 "$home/.ssh/authorized_keys"

if ! grep -qxF "$public_key" "$home/.ssh/authorized_keys"; then
  printf '%s\n' "$public_key" >> "$home/.ssh/authorized_keys"
  echo "the key of the panel is installed for $SSH_USER"
else
  echo "the key of the panel was already installed for $SSH_USER"
fi

# The panel is told that the key works, so it can reach this machine.
if ! curl -fsSL -X POST "$PANEL/install/confirm" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"nodeId\":\"$node_id\"}" >/dev/null; then
  echo "the panel was not told that the key works" >&2
  exit 1
fi

echo "the node $NAME is ready"
`

// Command builds the command an operator runs on a fresh machine.
func Command(panel, name, address, token string) string {
	return "curl -fsSL " + panel + "/install/ryxoui.sh | sh -s -- --panel " + panel +
		" --name " + name + " --address " + address + " --token " + token
}
