package main

import (
	"log"
	"socks5/socks5proxy"
)

func main() {
	go func() {
		server := socks5proxy.SOCKS5Server{
			IP:            "0.0.0.0",
			Port:          19080,
			SOCKS5Version: 0x05,
		}
		log.Fatal(server.Run())
	}()
	go func() {
		server := socks5proxy.SOCKS5Server{
			IP:            "0.0.0.0",
			Port:          19090,
			SOCKS5Version: 0x05,
		}
		log.Fatal(server.RunTLS())
	}()
	select {}
}
