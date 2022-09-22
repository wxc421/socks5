package socks5proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

const (
	SOCKS5Version = 0x05
)

type Server interface {
	Run() error
}

type SOCKS5Server struct {
	IP            string
	Port          int
	SOCKS5Version byte
}

type SOCKS5Context struct {
	conn      net.Conn // 客户端到SOCKS5服务器的连接
	target    net.Conn // SOCKS5服务器到代理服务器的连接
	r         *Request
	cid       uint // 请求id
	closeChan chan struct{}
}

func (s *SOCKS5Context) Close() error {
	f := func(conn io.Closer) {
		if conn != nil {
			_ = conn.Close()
		}
	}
	defer f(s.target)
	defer f(s.conn)
	defer log.Println(s.cid, ":conn will be closed")
	return nil
}

func (s *SOCKS5Context) auth() error {
	clientMessage, err := s.NewClientAuthMessage()
	if err != nil {
		return err
	}

	// Only support no-auth
	var acceptable bool
	for _, method := range clientMessage.Methods {
		if method == MethodNoAuth {
			acceptable = true
			break
		}
	}

	if !acceptable {
		return s.NewServerAuthMessage(MethodNoAcceptable)
	}
	return s.NewServerAuthMessage(MethodNoAuth)
}

// Response
// +----+-----+-------+------+----------+----------+
// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+
type Response struct {
	Version   byte
	ReplyCode byte
	AddrType  byte
	Port      [2]byte
	AddrDest  []byte
}

// REP代表响应状态码，值长度也是1个字节，有以下几种类型
// X’00’ succeeded
// X’01’ general SOCKS server failure
// X’02’ connection not allowed by ruleset
// X’03’ Network unreachable
// X’04’ Host unreachable
// X’05’ Connection refused
// X’06’ TTL expired
// X’07’ Command not supported
// X’08’ Address type not supported
// X’09’ to X’FF’ unassigned
const (
	Succeeded uint8 = iota
	SocksError
	ConnectionNotAllowed
	NetworkUnreachable
	ConnectionRefused
	TTLExpired
	CommandNotSupported
	AddressNotSupported
)

const (
	IPV4   byte = 0x01
	IPV6   byte = 0x04
	DOMAIN byte = 0x03
)

// write response to client to reply request
func (s *SOCKS5Context) write(resp *Response) error {
	// VER代表Socket协议的版本，Socks5默认为0x05，其值长度为1个字节
	// REP代表响应状态码，值长度也是1个字节
	// RSV保留字，值长度为1个字节
	// ATYP代表请求的远程服务器地址类型，值长度1个字节，有三种类型
	// IP V4 address: X’01’
	// DOMAINNAME: X’03’
	// IP V6 address: X’04’
	// BND.ADDR表示绑定地址，值长度不定。
	// BND.PORT表示绑定端口，值长度2个字节
	bytes := []byte{
		resp.Version,
		resp.ReplyCode,
		0x00,
		resp.AddrType,
	}
	bytes = append(bytes, resp.AddrDest...)
	bytes = append(bytes, resp.Port[:]...)
	_, err := s.conn.Write(bytes)
	return err
}

func (s *SOCKS5Context) error(replyCode byte) error {
	response := &Response{
		Version:   SOCKS5Version,
		ReplyCode: replyCode,
		AddrType:  0x00,
		Port:      [2]byte{0x00, 0x00},
		AddrDest:  []byte{0x00, 0x00, 0x00, 0x00},
	}
	return s.write(response)
}

func (s *SOCKS5Server) Run() error {
	address := fmt.Sprintf("%s:%d", s.IP, s.Port)
	listener, err := net.Listen("tcp", address)
	log.Println("server start")
	if err != nil {
		return err
	}
	var cid uint = 0
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("connection failure from %s: %s", conn.RemoteAddr(), err)
			continue
		}
		socks5Context := &SOCKS5Context{
			conn:      conn,
			cid:       cid,
			closeChan: make(chan struct{}, 1),
		}
		cid++
		log.Printf("Client connects to server %v\n", socks5Context.cid)
		go func(socks5Context *SOCKS5Context) {
			err := s.handle(socks5Context)
			if err != nil {
				log.Println(socks5Context.cid, " handle connection failure")
			}
		}(socks5Context)
	}
}

func (s *SOCKS5Server) RunTLS() error {
	crt, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Fatalln(err.Error())
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{crt},
	}
	address := fmt.Sprintf("%s:%d", s.IP, s.Port)
	// listener, err := net.Listen("tcp", address)
	listener, err := tls.Listen("tcp", address, tlsConfig)
	log.Println("server start")
	if err != nil {
		return err
	}
	var cid uint = 0
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("connection failure from %s: %s", conn.RemoteAddr(), err)
			continue
		}
		socks5Context := &SOCKS5Context{
			conn:      conn,
			cid:       cid,
			closeChan: make(chan struct{}, 1),
		}
		cid++
		log.Printf("Client connects to server %v\n", socks5Context.cid)
		go func(socks5Context *SOCKS5Context) {
			err := s.handle(socks5Context)
			if err != nil {
				log.Println(socks5Context.cid, " handle connection failure")
			}
		}(socks5Context)
	}
}

func Close(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		log.Println("close err")
	}
}

func (s *SOCKS5Server) handle(sc *SOCKS5Context) error {
	defer log.Println(sc.cid, " handle finish")
	defer Close(sc)
	// 协商过程
	if err := sc.auth(); err != nil {
		return err
	}
	log.Println(sc.cid, " auth success")
	// 请求过程
	if err := sc.request(); err != nil {
		return err
	}
	log.Println(sc.cid, " request success")
	// 转发过程
	if err := sc.proxy(); err != nil {
		return err
	}
	log.Println(sc.cid, " proxy success")
	return nil
}

// Request
// +----+-----+-------+------+----------+----------+
// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+
type Request struct {
	VER        byte
	CMD        byte
	RSV        byte
	ATYP       byte
	DST_ADDR   []byte
	DST_PORT   [2]byte
	DST_DOMAIN string
	RAW_ADDR   *net.TCPAddr
}

var (
	emptyStruct struct{}
)

func (s *SOCKS5Context) request() error {
	/**
	Once the method-dependent subnegotiation has completed, the client
	   sends the request details.  If the negotiated method includes
	   encapsulation for purposes of integrity checking and/or
	   confidentiality, these requests MUST be encapsulated in the method-dependent encapsulation.

	   The SOCKS request is formed as follows:



	     Where:

	          o  VER    protocol version: X'05'
	          o  CMD
	             o  CONNECT X'01'
	             o  BIND X'02'
	             o  UDP ASSOCIATE X'03'
	          o  RSV    RESERVED
	          o  ATYP   address type of following address
	             o  IP V4 address: X'01'
	             o  DOMAINNAME: X'03'
	             o  IP V6 address: X'04'
	          o  DST.ADDR       desired destination address
	          o  DST.PORT desired destination port in network octet
	             order

	   The SOCKS server will typically evaluate the request based on source
	   and destination addresses, and return one or more reply messages, as
	   appropriate for the request type.
	*/
	reader := bufio.NewReader(s.conn)
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		_ = s.error(SocksError)
		return errors.New("SocksError")
	}
	r := &Request{
		VER:  header[0],
		CMD:  header[1],
		RSV:  header[2],
		ATYP: header[3],
	}
	if r.VER != SOCKS5Version {
		log.Println("this protocol is not a socks 5 protocol")
		_ = s.error(SocksError)
		return errors.New("SocksError")
	}
	if r.CMD != 0x01 {
		log.Println("the client request type is not a proxy connection, and other functions are temporarily not supported")
		_ = s.error(SocksError)
		return errors.New("SocksError")
	}
	// DST.ADDR 就是目标地址的值了，
	// 如果是IPv4，那么就是4 bytes，
	// 如果是IPv6那么就是16 bytes，
	// 如果是域名，那么第一个字节代表接下来有多少个字节是表示目标地址
	switch r.ATYP {
	case IPV4:
		ipv4 := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(reader, ipv4); err != nil {
			_ = s.error(SocksError)
			return errors.New("SocksError")
		}
		r.DST_ADDR = ipv4
	case DOMAIN:
		length, err := reader.ReadByte()
		if err != nil {
			return s.error(SocksError)
		}
		domain := make([]byte, length)
		if _, err := io.ReadFull(reader, domain); err != nil {
			_ = s.error(SocksError)
			return errors.New("SocksError")
		}
		// Change to IP address to prevent DNS pollution and blocking
		ipAddr, err := net.ResolveIPAddr("ip", string(domain))
		log.Println(s.cid, " domain", string(domain))
		if err != nil {
			_ = s.error(SocksError)
			return errors.New("SocksError")
		}
		r.DST_ADDR = ipAddr.IP
	case IPV6:
		ipv6 := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(reader, ipv6); err != nil {
			_ = s.error(SocksError)
			return errors.New("SocksError")
		}
		r.DST_ADDR = ipv6
	default:
		_ = s.error(SocksError)
		return errors.New("SocksError")
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(reader, port); err != nil {
		_ = s.error(SocksError)
		return errors.New("SocksError")
	}
	r.DST_PORT = [2]byte{port[0], port[1]}
	r.RAW_ADDR = &net.TCPAddr{
		IP:   r.DST_ADDR,
		Port: int(binary.BigEndian.Uint16(port)),
	}
	s.r = r
	// establish a TCP connection
	return s.connect()
}

func (s *SOCKS5Context) proxy() error {
	wg := new(sync.WaitGroup)
	wg.Add(2)
	log.Println(s.cid, " connect success, start proxy")
	// 当 src close 之后, dst 就会收到 EOF 然后 return, 不会阻塞
	go func() {
		defer log.Printf("%v conn->target finish\n", s.cid)
		defer wg.Done()
		defer Close(s.target)
		log.Println(s.cid, " transfer conn to target")
		_, _ = io.Copy(s.target, s.conn)
	}()
	go func() {
		defer log.Printf("%v target->conn finish\n", s.cid)
		defer wg.Done()
		defer Close(s.conn)
		log.Println(s.cid, " transfer target to conn")
		_, _ = io.Copy(s.conn, s.target)
	}()
	wg.Wait()
	return nil
}

func (s *SOCKS5Context) connect() error {
	log.Println(s.cid, " connecting server:", s.r.RAW_ADDR)
	target, err := net.DialTCP("tcp", nil, s.r.RAW_ADDR)
	if err != nil {
		log.Println(s.cid, " Failed to connect to remote server:", s.r.RAW_ADDR, " err: ", err.Error())
		if strings.Contains(err.Error(), "refused") {
			_ = s.error(ConnectionRefused)
			return errors.New("ConnectionRefused")
		} else if strings.Contains(err.Error(), "network is unreachable") {
			_ = s.error(NetworkUnreachable)
			return errors.New("NetworkUnreachable")
		} else {
			_ = s.error(ConnectionRefused)
			return errors.New("ConnectionRefused")
		}
	}
	log.Println(s.cid, " connect to remote server:", s.r.RAW_ADDR, "success ")
	s.target = target
	local := target.LocalAddr().(*net.TCPAddr)
	// socks5既充当socks服务器，又充当relay服务器。
	// 实际上这两个是可以被拆开的，当我们的socks5 server和relay server不是一体的，
	// 就需要告知客户端relay server的地址，这个地址就是BND.ADDR和BND.PORT。
	// 当我们的relay server和socks5 server是同一台服务器时，BND.ADDR和BND.PORT的值全部为0即可。
	response := &Response{
		Version:   SOCKS5Version,
		ReplyCode: Succeeded,
		AddrType:  0,
		Port:      [2]byte{byte(local.Port >> 8), byte(local.Port & 0xff)},
		AddrDest:  nil,
	}
	// parse ip
	if local.IP.To4() != nil {
		response.AddrType = IPV4
		response.AddrDest = local.IP.To4()
	} else if local.IP.To16() != nil {
		response.AddrType = IPV6
		response.AddrDest = local.IP.To16()
	} else {
		_ = s.error(AddressNotSupported)
		return errors.New("AddressNotSupported")
	}

	if err := s.write(response); err != nil {
		return err
	}
	return nil
}
