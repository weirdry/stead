package tailscale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Peer struct {
	HostName string
	DNSName  string
	IP       string
	Online   bool
}

type statusJSON struct {
	Peer map[string]peerJSON `json:"Peer"`
}

type peerJSON struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
}

func DiscoverPeer(alias string) (Peer, error) {
	path, err := tailscalePath()
	if err != nil {
		return Peer{}, err
	}
	out, err := exec.Command(path, "status", "--json").CombinedOutput()
	if err != nil {
		return Peer{}, fmt.Errorf("tailscale status --json failed: %w", err)
	}
	out = bytes.TrimSpace(out)
	if !bytes.HasPrefix(out, []byte("{")) {
		return Peer{}, fmt.Errorf("tailscale status --json did not return JSON; install or enable the Tailscale CLI")
	}
	return ParsePeer(out, alias)
}

func ParsePeer(data []byte, alias string) (Peer, error) {
	var status statusJSON
	if err := json.Unmarshal(data, &status); err != nil {
		return Peer{}, err
	}

	matches := make([]Peer, 0)
	for _, peer := range status.Peer {
		candidate := convert(peer)
		if matchesAlias(candidate, alias) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return Peer{}, fmt.Errorf("no Tailscale peer matched alias %q", alias)
	case 1:
		return matches[0], nil
	default:
		return Peer{}, fmt.Errorf("multiple Tailscale peers matched alias %q", alias)
	}
}

func (p Peer) Hostname() string {
	if p.DNSName != "" {
		return strings.TrimSuffix(p.DNSName, ".")
	}
	return p.IP
}

func convert(peer peerJSON) Peer {
	ip := ""
	if len(peer.TailscaleIPs) > 0 {
		ip = peer.TailscaleIPs[0]
	}
	return Peer{
		HostName: peer.HostName,
		DNSName:  strings.TrimSuffix(peer.DNSName, "."),
		IP:       ip,
		Online:   peer.Online,
	}
}

func matchesAlias(peer Peer, alias string) bool {
	alias = strings.ToLower(strings.TrimSpace(alias))
	if alias == "" {
		return false
	}
	hostName := strings.ToLower(peer.HostName)
	dnsName := strings.ToLower(strings.TrimSuffix(peer.DNSName, "."))
	return hostName == alias || dnsName == alias || strings.HasPrefix(dnsName, alias+".")
}

func tailscalePath() (string, error) {
	if path, err := exec.LookPath("tailscale"); err == nil {
		return path, nil
	}
	for _, path := range []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/Applications/Tailscale.app/Contents/MacOS/tailscale",
	} {
		if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("tailscale CLI not found")
}
