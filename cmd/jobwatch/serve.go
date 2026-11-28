package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/api"
	"github.com/stepbeta/jobwatch/web"
)

func newServeCmd(opts *rootOptions) *cobra.Command {
	var port int
	var addr string
	var unsafeBind bool
	var allowHosts []string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local web server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			warning, err := bindPolicy(addr, unsafeBind)
			if err != nil {
				return err
			}
			if warning != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), warning)
			}

			// Resolve the accepted Host values before the database is
			// opened, so a bad --allow-host cannot leave a half-started
			// server behind.
			hostname, _ := os.Hostname()
			allowedHosts, err := hostAllowlist(addr, allowHosts, interfaceAddrs(), hostname)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			mux := http.NewServeMux()
			mux.Handle("/api/", (&api.Server{DB: sqlDB, Version: version, AllowedHosts: allowedHosts}).Handler())
			mux.Handle("/", web.Handler())

			srv := &http.Server{
				Addr:              listenAddr(addr, port),
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
			}
			errc := make(chan error, 1)
			go func() { errc <- srv.ListenAndServe() }()
			fmt.Fprintf(cmd.OutOrStdout(), "JobWatch listening on http://%s (db %s)\n", srv.Addr, opts.dbPath)

			select {
			case err := <-errc:
				return fmt.Errorf("serve: %w", err)
			case <-ctx.Done():
			}
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("shutdown: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "TCP port to listen on")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1", "address to bind (localhost only by default)")
	cmd.Flags().BoolVar(&unsafeBind, "unsafe-bind", false, "allow binding a non-loopback address (the API has no authentication)")
	cmd.Flags().StringArrayVar(&allowHosts, "allow-host", nil, "extra host name the API accepts, such as a proxy or tunnel domain (repeatable)")
	return cmd
}

// listenAddr joins a bind address with the port. Bracketed IPv6 input
// such as [::1] is accepted on the command line and normalized so that
// net.JoinHostPort does not produce [[::1]]:port.
func listenAddr(addr string, port int) string {
	return net.JoinHostPort(strings.Trim(strings.TrimSpace(addr), "[]"), strconv.Itoa(port))
}

// bindPolicy validates --addr before the database is opened. It returns a
// warning to print when the operator explicitly allows a non-loopback bind.
func bindPolicy(addr string, unsafeBind bool) (string, error) {
	if isLoopbackBindAddr(addr) {
		return "", nil
	}
	if !unsafeBind {
		return "", fmt.Errorf("refusing to bind %q: the API has no authentication; pass --unsafe-bind to expose it", addr)
	}
	return fmt.Sprintf("warning: binding %q exposes the API to the network; it has no authentication", addr), nil
}

// isLoopbackBindAddr reports whether addr names loopback: localhost,
// 127.0.0.0/8 or ::1. An empty or wildcard address is not loopback.
func isLoopbackBindAddr(addr string) bool {
	addr = strings.TrimSpace(addr)
	if strings.EqualFold(addr, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(addr, "[]"))
	return ip != nil && ip.IsLoopback()
}

// isWildcardBindAddr reports whether addr asks the listener to bind every
// interface: an unspecified address (0.0.0.0, ::) or an empty value.
func isWildcardBindAddr(addr string) bool {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return true
	}
	ip := net.ParseIP(strings.Trim(addr, "[]"))
	return ip != nil && ip.IsUnspecified()
}

// interfaceAddrs returns the machine's unicast interface addresses as text.
// An enumeration failure yields no extra entries, so loopback and explicit
// --allow-host values keep working.
func interfaceAddrs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP
		if ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		out = append(out, ip.String())
	}
	return out
}

// hostAllowlist builds the Host values the API accepts besides loopback and
// *.localhost: the --allow-host entries, the bound address when it is
// specific, and, for a wildcard bind, the machine's own interface addresses
// and hostname. Entries are normalized like API hosts and deduplicated.
func hostAllowlist(bindAddr string, explicit, ifaceAddrs []string, hostname string) ([]string, error) {
	out := make([]string, 0, len(explicit)+len(ifaceAddrs)+2)
	seen := make(map[string]bool, len(explicit)+len(ifaceAddrs)+2)
	add := func(host string) {
		if host == "" || seen[host] {
			return
		}
		seen[host] = true
		out = append(out, host)
	}
	for _, value := range explicit {
		host, err := parseAllowHost(value)
		if err != nil {
			return nil, err
		}
		add(host)
	}
	if isWildcardBindAddr(bindAddr) {
		for _, a := range ifaceAddrs {
			add(api.NormalizeHost(a))
		}
		add(api.NormalizeHost(hostname))
	} else {
		add(api.NormalizeHost(bindAddr))
	}
	return out, nil
}

// parseAllowHost validates one --allow-host value and returns the normalized
// host. Only a host name or IP address is accepted, with an optional port.
func parseAllowHost(value string) (string, error) {
	raw := strings.TrimSpace(value)
	switch {
	case raw == "":
		return "", errors.New("--allow-host needs a host name")
	case strings.Contains(raw, "://"):
		return "", fmt.Errorf("--allow-host %q must be a host name, not a URL", value)
	case strings.ContainsAny(raw, "/?#"):
		return "", fmt.Errorf("--allow-host %q must not carry a path or query", value)
	}
	if host, port, err := net.SplitHostPort(raw); err == nil {
		if port != "" {
			if _, err := strconv.Atoi(port); err != nil {
				return "", fmt.Errorf("--allow-host %q has a non-numeric port", value)
			}
		}
		raw = host
	} else if strings.Contains(raw, ":") && net.ParseIP(strings.Trim(raw, "[]")) == nil {
		return "", fmt.Errorf("--allow-host %q is not a host name or IP address", value)
	}
	host := api.NormalizeHost(raw)
	if host == "" || strings.ContainsAny(host, "* ") {
		return "", fmt.Errorf("--allow-host %q is not a host name or IP address", value)
	}
	return host, nil
}
