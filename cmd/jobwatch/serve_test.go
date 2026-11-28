package main

import (
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
