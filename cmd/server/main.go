package main

import (
	"log"
	"runtime"
	"socks5/socks5proxy"
)

func main() {
	runtime.GOMAXPROCS(1)
	server := socks5proxy.SOCKS5Server{
		IP:            "0.0.0.0",
		Port:          9080,
		SOCKS5Version: 0x05,
	}
	log.Fatal(server.Run())
}
