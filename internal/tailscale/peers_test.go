package tailscale

import (
	"strings"
	"testing"
)

func TestParsePeerUsesDNSName(t *testing.T) {
	peer, err := ParsePeer([]byte(`{
  "Peer": {
    "nodeid:1": {
      "HostName": "devmac",
      "DNSName": "devmac.tailnet.example.",
      "TailscaleIPs": ["ts-ip"],
      "Online": true
    }
  }
}`), "devmac")
	if err != nil {
		t.Fatalf("ParsePeer returned error: %v", err)
	}
	if peer.Hostname() != "devmac.tailnet.example" {
		t.Fatalf("hostname = %q", peer.Hostname())
	}
	if !peer.Online {
		t.Fatal("peer should be online")
	}
}

func TestParsePeerFallsBackToIP(t *testing.T) {
	peer, err := ParsePeer([]byte(`{
  "Peer": {
    "nodeid:1": {
      "HostName": "devmac",
      "TailscaleIPs": ["ts-ip"]
    }
  }
}`), "devmac")
	if err != nil {
		t.Fatalf("ParsePeer returned error: %v", err)
	}
	if peer.Hostname() != "ts-ip" {
		t.Fatalf("hostname = %q", peer.Hostname())
	}
}

func TestParsePeerReportsNoMatch(t *testing.T) {
	_, err := ParsePeer([]byte(`{"Peer":{}}`), "devmac")
	if err == nil {
		t.Fatal("expected no match error")
	}
	if !strings.Contains(err.Error(), "no Tailscale peer matched") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePeerRejectsInvalidJSON(t *testing.T) {
	_, err := ParsePeer([]byte(`not json`), "devmac")
	if err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestParsePeerReportsMultipleMatches(t *testing.T) {
	_, err := ParsePeer([]byte(`{
  "Peer": {
    "nodeid:1": {"HostName": "devmac", "TailscaleIPs": ["ts-ip-a"]},
    "nodeid:2": {"DNSName": "devmac.tailnet.example.", "TailscaleIPs": ["ts-ip-b"]}
  }
}`), "devmac")
	if err == nil {
		t.Fatal("expected multiple match error")
	}
	if !strings.Contains(err.Error(), "multiple Tailscale peers matched") {
		t.Fatalf("unexpected error: %v", err)
	}
}
