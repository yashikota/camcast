package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
)

func main() {
	// HTTPS -> HTTP director
	const publishSeverScheme = "http"
	director := func(req *http.Request) {
		req.URL.Scheme = publishSeverScheme
		req.URL.Host = ":" + getPortNumber(publishSeverScheme)
	}

	// ReverseProxy
	rp := &httputil.ReverseProxy{Director: director}

	// ReverseProxy server
	const ReverseProxyServerScheme = "https"
	httpsServer := http.Server{
		Addr:    ":" + getPortNumber(ReverseProxyServerScheme),
		Handler: rp,
	}

	dir := ".certs"
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Check if cert.pem and key.pem exist
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	if os.IsNotExist(certErr) || os.IsNotExist(keyErr) {
		// Generate self-signed certificate
		generateCert()
	}

	// Check if mediamtx folder exists
	_, err := os.Stat("mediamtx")
	if os.IsNotExist(err) {
		downloadMediaMTX()
	}

	// Start MediaMTX server
	go func() {
		launchMediaMTX()
	}()

	// Open browser
	openBrowser()

	// Show IP address
	ip, err := LocalIP()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("https://" + ip.String() + ":" + getPortNumber(ReverseProxyServerScheme))

	// Start HTTPS server
	fmt.Println("Server started at", "http://localhost:"+getPortNumber(publishSeverScheme))
	if err := httpsServer.ListenAndServeTLS(certPath, keyPath); err != nil {
		log.Fatal(err)
	}
}

func getPortNumber(scheme string) string {
	if scheme == "http" {
		return "8889"
	} else {
		return "8443"
	}
}
