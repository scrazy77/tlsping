// Package tlsping measures the time needed for establishing TLS connections
package tlsping

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"sync"
	"time"
)

// Config is used to configure how to time the TLS connection
type Config struct {
	// Dont perform TLS handshake. Only measure the time for
	//  estasblishing the TCP connection
	AvoidTLSHandshake bool

	// Don't verify server certificate. Used relevant if
	// the TLS handshake is performed
	InsecureSkipVerify bool

	// Set of root certificate authorities to use to verify the server
	// certificate. This is only relevant when measuring the time spent
	// in the TLS handshake.
	// If nil, the host's set of root certificate authorities is used.
	RootCAs *x509.CertPool

	// Number of times to connect. The time spent by every connection will
	// be measured and the results will be summarized.
	Count int

	Ip string
}

// Ping establishes network connections to the specified network addr
// and returns summary statistics of the time spent establishing those
// connections. The operation is governed by the provided configuration.
// It returns an error if at least one of the connections fails.
// addr is of the form 'hostname:port'
// The returned results do not include the time spent calling the
// DNS for translating the host name to IP address. This resolution
// is performed once and a single of retrieved IP addresses is used for all
// connections.
func Ping(addr string, config *Config) (PingResult, error) {
	if config.Count == 0 {
		config.Count = 1
	}
	host, ipAddr, port, err := resolveAddr(addr)
	if err != nil {
		return PingResult{}, err
	}
	result := PingResult{
		Host:    host,
		IPAddr:  ipAddr,
		Address: addr,
	}
	target := net.JoinHostPort(ipAddr, port)
	var f func() error
	d := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	if config.AvoidTLSHandshake {
		f = func() error {
			conn, err := d.Dial("tcp", target)
			if err == nil {
				conn.Close()
			}
			return err
		}
	} else {
		tlsConfig := tls.Config{
			ServerName:         host,
			InsecureSkipVerify: config.InsecureSkipVerify,
			RootCAs:            config.RootCAs,
		}
		f = func() error {
			conn, err := tls.DialWithDialer(d, "tcp", target, &tlsConfig)
			if err == nil {
				conn.Close()
			}
			return err
		}
	}

	// Launch workers to perform the timing
	results := make(chan connectDuration, config.Count)
	var wg sync.WaitGroup
	wg.Add(config.Count)
	for i := 0; i < config.Count; i++ {
		go func() {
			defer wg.Done()
			d, err := timeit(f)
			results <- connectDuration{
				seconds: d,
				err:     err,
			}
		}()
	}

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect workers' results
	durations := make([]float64, 0, config.Count)
	for res := range results {
		if res.err != nil {
			return result, res.err
		}
		durations = append(durations, res.seconds)
	}
	result.setSummaryStats(summarize(durations))
	return result, nil
}

type connectDuration struct {
	seconds float64
	err     error
}

// timeit measures the time spent executing the argument function f
// It returns the elapsed time spent as a floating point number of seconds
func timeit(f func() error) (float64, error) {
	start := time.Now()
	err := f()
	end := time.Now()
	if err != nil {
		return 0, err
	}
	return end.Sub(start).Seconds(), nil
}

// resolveAddr queries the DNS to resolve the name of the host
// in addr and returns the hostname, IP address and port.
// If the DNS responds with more than one IP address associated
// to the given host, the first address to which a TCP connection
// can be established is returned.
func resolveAddr(addr string) (string, string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", "", err
	}
	if len(host) == 0 {
		host = "localhost"
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return "", "", "", err
	}
	d := net.Dialer{Timeout: 3 * time.Second}
	for _, a := range addrs {
		conn, err := d.Dial("tcp", net.JoinHostPort(a, port))
		if err == nil {
			conn.Close()
			return host, a, port, nil
		}
	}
	return host, addrs[0], port, nil
}
