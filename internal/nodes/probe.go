package nodes

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ryxoui/ryxoui/internal/ssh"
)

// ErrNoKey is returned when a node has no key the panel can use.
var ErrNoKey = errors.New("nodes: the node has no usable key")

// ErrKeyNotInstalled is returned when the key of a node never reached it.
var ErrKeyNotInstalled = errors.New("nodes: the key of the node is not installed yet")

// ProbeResult is what a node reports about itself.
type ProbeResult struct {
	OS          string  `json:"os"`
	Arch        string  `json:"arch"`
	Kernel      string  `json:"kernel"`
	CoreVersion string  `json:"coreVersion"`
	NodeVersion string  `json:"nodeVersion"`
	DiskTotal   int64   `json:"diskTotal"`
	DiskUsed    int64   `json:"diskUsed"`
	MemTotal    int64   `json:"memTotal"`
	MemUsed     int64   `json:"memUsed"`
	Load1       float64 `json:"load1"`
	HostKey     string  `json:"-"`
}

// HealthSample builds a health sample from a probe result.
func (r ProbeResult) HealthSample(nodeID string, at int64) HealthSample {
	return HealthSample{
		NodeID:    nodeID,
		DiskTotal: r.DiskTotal,
		DiskUsed:  r.DiskUsed,
		MemTotal:  r.MemTotal,
		MemUsed:   r.MemUsed,
		Load1:     r.Load1,
		SampledAt: at,
	}
}

// probeCommand is the script the panel runs on a node. It prints one
// key=value pair per line and never fails because a value is missing.
const probeCommand = `set -u
printf 'os=%s\n' "$(uname -s 2>/dev/null | tr 'A-Z' 'a-z')"
printf 'arch=%s\n' "$(uname -m 2>/dev/null)"
printf 'kernel=%s\n' "$(uname -r 2>/dev/null)"
if [ -r /proc/meminfo ]; then
  printf 'mem_total=%s\n' "$(( $(awk '/^MemTotal/{print $2}' /proc/meminfo) * 1024 ))"
  printf 'mem_available=%s\n' "$(( $(awk '/^MemAvailable/{print $2}' /proc/meminfo) * 1024 ))"
fi
if command -v df >/dev/null 2>&1; then
  printf 'disk_total=%s\n' "$(df -k / 2>/dev/null | awk 'NR==2{print $2*1024}')"
  printf 'disk_used=%s\n' "$(df -k / 2>/dev/null | awk 'NR==2{print $3*1024}')"
fi
if [ -r /proc/loadavg ]; then
  printf 'load1=%s\n' "$(awk '{print $1}' /proc/loadavg 2>/dev/null)"
fi
for binary in ryxoui xray sing-box; do
  if command -v "$binary" >/dev/null 2>&1; then
    printf '%s_version=%s\n' "$binary" "$("$binary" version 2>/dev/null | head -n 1)"
  fi
done`

// Probe connects to a node and asks it about itself.
//
// The host key of the node is returned with the result so the panel can pin
// it after the first successful connection.
func Probe(ctx context.Context, node Node, privateKey string) (ProbeResult, error) {
	if strings.TrimSpace(privateKey) == "" {
		return ProbeResult{}, ErrNoKey
	}

	client, err := ssh.NewClient(ssh.Config{
		Host:       node.SSHHostName(),
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: privateKey,
		KnownHost:  node.HostKey,
	})
	if err != nil {
		return ProbeResult{}, err
	}

	session, err := client.Connect(ctx)
	if err != nil {
		return ProbeResult{}, err
	}
	defer session.Close()

	stdout, stderr, code, err := session.Run(ctx, probeCommand)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("nodes: run the probe: %w", err)
	}
	if code != 0 {
		return ProbeResult{}, fmt.Errorf("nodes: the probe failed with code %d: %s", code, strings.TrimSpace(stderr))
	}

	result := parseProbeOutput(stdout)
	result.HostKey = client.KnownHost()
	return result, nil
}

// parseProbeOutput reads the key=value pairs the probe prints.
func parseProbeOutput(output string) ProbeResult {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	result := ProbeResult{
		OS:          values["os"],
		Arch:        values["arch"],
		Kernel:      values["kernel"],
		CoreVersion: values["xray_version"],
		NodeVersion: values["ryxoui_version"],
		DiskTotal:   parseBytes(values["disk_total"]),
		DiskUsed:    parseBytes(values["disk_used"]),
		MemTotal:    parseBytes(values["mem_total"]),
		Load1:       parseFloat(values["load1"]),
	}
	if result.CoreVersion == "" {
		result.CoreVersion = values["sing-box_version"]
	}
	if available := parseBytes(values["mem_available"]); available > 0 && result.MemTotal > available {
		result.MemUsed = result.MemTotal - available
	}
	return result
}

// parseBytes reads a number of bytes.
func parseBytes(value string) int64 {
	number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || number < 0 {
		return 0
	}
	return number
}

// parseFloat reads a floating point number.
func parseFloat(value string) float64 {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return number
}
