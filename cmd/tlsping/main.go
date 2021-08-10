package main

import (
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/airnandez/tlsping"
)

func main() {
	fset := flag.NewFlagSet(appName, flag.ExitOnError)
	fset.Usage = func() {
		printUsage(os.Stderr, usageShort)
	}
	tcpOnly := fset.Bool("tcponly", false, "")
	count := fset.Int("c", defaultIterations, "")
	jsonOutput := fset.Bool("json", false, "")
	insecure := fset.Bool("insecure", false, "")
	ca := fset.String("ca", "", "")
	version := fset.Bool("version", false, "")
	help := fset.Bool("help", false, "")
	ip := fset.String("ip", "", "")
	fset.Parse(os.Args[1:])

	if *version {
		printVersion(os.Stderr)
		os.Exit(0)
	}
	if *help {
		printUsage(os.Stderr, usageLong)
		os.Exit(0)
	}
	args := fset.Args()
	if len(args) != 1 {
		errlog.Printf("missing server address\n")
		printUsage(os.Stderr, usageShort)
		os.Exit(1)
	}
	serverAddr := args[0]
	if *count <= 0 {
		*count = 1
	}
	if *count > maxCount {
		errlog.Printf("number of allowed connections cannot exceed %d\n", maxCount)
		printUsage(os.Stderr, usageShort)
		os.Exit(1)
	}
	caCerts, err := loadCaCerts(*ca)
	if err != nil {
		errlog.Printf("%s\n", err)
		printUsage(os.Stderr, usageShort)
		os.Exit(1)
	}
	config := tlsping.Config{
		Count:              *count,
		AvoidTLSHandshake:  *tcpOnly,
		InsecureSkipVerify: *insecure,
		RootCAs:            caCerts,
		Ip:                 ip,
	}
	result, err := tlsping.Ping(serverAddr, &config)
	if err != nil {
		errlog.Printf("error connecting to '%s': %s\n", serverAddr, err)
		os.Exit(1)
	}
	s := "TLS"
	if *tcpOnly {
		s = "TCP"
	}
	if !*jsonOutput {
		outlog.Printf("%s connection to %s (%s) (%d connections)\n", s, serverAddr, result.IPAddr, *count)
		outlog.Printf("min/avg/max/stddev = %s/%s/%s/%s\n", result.MinStr(), result.AvgStr(), result.MaxStr(), result.StdStr())
		os.Exit(0)
	}

	// Format the result in JSON
	jsonRes := JsonResult{
		Host:       result.Host,
		IPAddr:     result.IPAddr,
		ServerAddr: result.Address,
		Connection: s,
		Min:        result.Min,
		Max:        result.Max,
		Count:      result.Count,
		Avg:        result.Avg,
		Std:        result.Std,
	}
	if err != nil {
		jsonRes.Error = fmt.Sprintf("%s", err)
	}
	b, err := json.Marshal(jsonRes)
	if err != nil {
		errlog.Printf("error producing JSON: %s\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
	os.Exit(0)
}

type JsonResult struct {
	Host       string  `json:"host"`
	IPAddr     string  `json:"ip"`
	ServerAddr string  `json:"address"`
	Connection string  `json:"connection"`
	Count      int     `json:"count"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Avg        float64 `json:"average"`
	Std        float64 `json:"stddev"`
	Error      string  `json:"error"`
}

func loadCaCerts(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	caCerts, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error loading CA certficates from '%s': %s", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCerts) {
		return nil, fmt.Errorf("error creating pool of CA certficates: %s", err)
	}
	return pool, nil
}
