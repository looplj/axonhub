package server

import "testing"

func TestListenAddr(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{host: "127.0.0.1", port: 18090, want: "127.0.0.1:18090"},
		{host: "0.0.0.0", port: 8090, want: "0.0.0.0:8090"},
		{host: "localhost", port: 18090, want: "localhost:18090"},
		{host: "::1", port: 18090, want: "[::1]:18090"},
		{host: "", port: 18090, want: ":18090"},
	}
	for _, tt := range tests {
		if got := listenAddr(tt.host, tt.port); got != tt.want {
			t.Errorf("listenAddr(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
		}
	}
}
