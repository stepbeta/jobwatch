package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestListenAddr(t *testing.T) {
	tests := []struct {
		addr string
		want string
	}{
		{"127.0.0.1", "127.0.0.1:8080"},
		{"localhost", "localhost:8080"},
		{"::1", "[::1]:8080"},
		{"[::1]", "[::1]:8080"},
		{"0.0.0.0", "0.0.0.0:8080"},
	}
	for _, tt := range tests {
		if got := listenAddr(tt.addr, 8080); got != tt.want {
			t.Errorf("listenAddr(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}

func TestBindPolicy(t *testing.T) {
	tests := []struct {
		addr        string
		unsafeBind  bool
		wantErr     bool
		wantWarning bool
	}{
		{"localhost", false, false, false},
		{"127.0.0.1", false, false, false},
		{"::1", false, false, false},
		{"[::1]", false, false, false},
		{"0.0.0.0", false, true, false},
		{"0.0.0.0", true, false, true},
		{"192.168.1.20", false, true, false},
		{"192.168.1.20", true, false, true},
		{"", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			warning, err := bindPolicy(tt.addr, tt.unsafeBind)
			if (err != nil) != tt.wantErr {
				t.Fatalf("bindPolicy(%q, %v) error = %v, wantErr %v", tt.addr, tt.unsafeBind, err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "--unsafe-bind") {
				t.Errorf("error %q does not mention --unsafe-bind", err)
			}
			if (warning != "") != tt.wantWarning {
				t.Errorf("warning = %q, wantWarning %v", warning, tt.wantWarning)
			}
			if tt.wantWarning && !strings.Contains(warning, "no authentication") {
				t.Errorf("warning %q does not name the missing authentication", warning)
			}
		})
	}
}

func TestParseAllowHost(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{"host name", "proxy.example", "proxy.example", false},
		{"host and port", "Proxy.Example:443", "proxy.example", false},
		{"surrounding space and trailing dot", "  internal.example.  ", "internal.example", false},
		{"IPv4", "192.168.1.20", "192.168.1.20", false},
		{"IPv6 with port", "[::1]:8080", "::1", false},
		{"bare IPv6", "::1", "::1", false},
		{"empty", "", "", true},
		{"blank", "   ", "", true},
		{"URL", "https://internal.example", "", true},
		{"path", "internal.example/app", "", true},
		{"query", "internal.example?x=1", "", true},
		{"wildcard", "*", "", true},
		{"inner space", "exa mple", "", true},
		{"bad port", "example.com:notaport", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAllowHost(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseAllowHost(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), "--allow-host") {
					t.Errorf("error %q does not name --allow-host", err)
				}
				return
			}
			if got != tt.want {
				t.Errorf("parseAllowHost(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestHostAllowlist(t *testing.T) {
	iface := []string{"127.0.0.1", "192.168.1.73", "fe80::1"}
	wildcard := []string{"127.0.0.1", "192.168.1.73", "fe80::1", "marie-curie"}

	tests := []struct {
		name     string
		bind     string
		explicit []string
		hostname string
		want     []string
	}{
		{"loopback bind", "127.0.0.1", nil, "marie-curie", []string{"127.0.0.1"}},
		{"specific bind with an explicit entry", "192.168.1.20", []string{"Proxy.Example:443"}, "marie-curie",
			[]string{"proxy.example", "192.168.1.20"}},
		{"wildcard IPv4", "0.0.0.0", nil, "MARIE-CURIE", wildcard},
		{"wildcard IPv6", "::", nil, "marie-curie", wildcard},
		{"empty bind is wildcard", "", nil, "marie-curie", wildcard},
		{"deduplicated entries", "0.0.0.0", []string{"192.168.1.73", "MARIE-CURIE"}, "marie-curie",
			[]string{"192.168.1.73", "marie-curie", "127.0.0.1", "fe80::1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hostAllowlist(tt.bind, tt.explicit, iface, tt.hostname)
			if err != nil {
				t.Fatalf("hostAllowlist error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hostAllowlist(%q) = %v, want %v", tt.bind, got, tt.want)
			}
		})
	}

	if _, err := hostAllowlist("127.0.0.1", []string{"https://proxy.example"}, iface, ""); err == nil {
		t.Fatal("hostAllowlist accepted an invalid --allow-host value")
	}
}

func TestServeRejectsInvalidAllowHostBeforeOpeningDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jobwatch.db")
	_, err := runCLI(t, dbPath, "serve", "--allow-host", "https://proxy.example")
	if err == nil {
		t.Fatal("serve accepted a URL as --allow-host")
	}
	if !strings.Contains(err.Error(), "--allow-host") {
		t.Errorf("error %q does not name --allow-host", err)
	}
	if _, statErr := os.Stat(dbPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("database %s was opened before the flag was validated", dbPath)
	}
}
