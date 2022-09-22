package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {

	l, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalln(err.Error())
	}
	for {
		conn, err := l.Accept()
		fmt.Println("accept conn")
		if err != nil {
			fmt.Println(err.Error())
			continue
		} else {
			go HandleClientConnect(conn)
		}
	}
}

func HandleClientConnect(conn net.Conn) {
	defer conn.Close()
	conf := &tls.Config{
		InsecureSkipVerify: true, // 这里是跳过证书验证，因为证书签发机构的CA证书是不被认证的
	}
	// 注意这里要使用证书中包含的主机名称
	conn2, err := tls.Dial("tcp", "127.0.0.1:9080", conf)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer conn2.Close()
	go io.Copy(conn, conn2)
	io.Copy(conn2, conn)
}
