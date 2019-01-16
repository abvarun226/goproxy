package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli"
)

// Default Timeouts
const (
	DialTimeout  = 15 * time.Second
	ReadTimeout  = 15 * time.Second
	WriteTimeout = 15 * time.Second
)

// Default String Values
const (
	TCP                   = "tcp"
	HijackingNotSupported = "hijacking not supported"
)

func tunnel(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func tcpConnectHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[TCPConnect Handler] URI=%s", r.RequestURI)

	destConn, err := net.DialTimeout(TCP, r.Host, DialTimeout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, HijackingNotSupported, http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	go tunnel(destConn, clientConn)
	go tunnel(clientConn, destConn)
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP Handler] URI=%s", r.RequestURI)

	rsp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	defer rsp.Body.Close()

	copyHeaders(w.Header(), rsp.Header)
	w.WriteHeader(rsp.StatusCode)
	io.Copy(w, rsp.Body)
}

func copyHeaders(destination, source http.Header) {
	for k, v1 := range source {
		for _, v2 := range v1 {
			destination.Add(k, v2)
		}
	}
}

func handleFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		tcpConnectHandler(w, r)
	} else {
		httpHandler(w, r)
	}
}

func main() {
	var proxyPort int
	app := cli.NewApp()
	app.Name = "goproxy"
	app.Usage = "Golang based proxy"
	app.Version = "1.0.0"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:   "port, p",
			Value:  8080,
			Usage:  "Proxy port",
			EnvVar: "PORT",
		},
	}
	app.Action = func(c *cli.Context) {
		proxyPort = c.Int("port")
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("[INFO] Running proxy on port %d", proxyPort)

	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", proxyPort),
		Handler:        http.HandlerFunc(handleFunc),
		ReadTimeout:    ReadTimeout,
		WriteTimeout:   WriteTimeout,
		MaxHeaderBytes: 1 << 20,
		TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	log.Fatal(server.ListenAndServe())
}
