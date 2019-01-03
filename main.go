package main

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

func tunnel(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func tcpConnectHandler(w http.ResponseWriter, r *http.Request) {
	log.Print(r.RequestURI)

	destConn, err := net.DialTimeout("tcp", r.Host, 15*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
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
	log.Print(r.RequestURI)

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
	server := &http.Server{
		Addr:           ":8080",
		Handler:        http.HandlerFunc(handleFunc),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSNextProto:   make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	log.Fatal(server.ListenAndServe())
}
