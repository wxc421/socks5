package socks5proxy

import (
	"errors"
	"io"
)

type ClientAuthMessage struct {
	Version  byte
	NMethods byte
	Methods  []Method
}
type Method = byte

const (
	MethodNoAuth       Method = 0x00
	MethodGSSAPI       Method = 0x01
	MethodPassword     Method = 0x02
	MethodNoAcceptable Method = 0xff
)

func (s *SOCKS5Context) NewClientAuthMessage() (*ClientAuthMessage, error) {
	/**
	    The localConn connects to the dstServer, and sends a ver
	    identifier/method selection message:
	                +----+----------+----------+
	                |VER | NMETHODS | METHODS  |
	                +----+----------+----------+
	                | 1  |    1     | 1 to 255 |
	                +----+----------+----------+
	    The VER field is set to X'05' for this ver of the protocol.  The
	    NMETHODS field contains the number of method identifier octets that
	    appear in the METHODS field.
	    METHODS常见的几种方式如下:
	    1>.数字“0”：表示不需要用户名或者密码验证；
	    2>.数字“1”：GSSAPI是SSH支持的一种验证方式；
	    3>.数字“2”：表示需要用户名和密码进行验证；
	    4>.数字“3”至“7F”：表示用于IANA 分配(IANA ASSIGNED)
	    5>.数字“80”至“FE”表示私人方法保留(RESERVED FOR PRIVATE METHODS)
	    4>.数字“FF”：不支持所有的验证方式，无法进行连接
	**/
	buf := make([]byte, 2)
	_, err := io.ReadFull(s.conn, buf)
	if err != nil {
		return nil, err
	}

	// Validate version
	if buf[0] != SOCKS5Version {
		return nil, errors.New("protocol version not supported")
	}

	// Read methods
	nmethods := buf[1]
	buf = make([]byte, nmethods)
	_, err = io.ReadFull(s.conn, buf)
	if err != nil {
		return nil, err
	}
	c := &ClientAuthMessage{
		Version:  SOCKS5Version,
		NMethods: nmethods,
		Methods:  buf,
	}
	return c, nil
}

func (s *SOCKS5Context) NewServerAuthMessage(method Method) error {
	//                  +----+--------+
	//                 |VER | METHOD |
	//                 +----+--------+
	//                 | 1  |   1    |
	//                 +----+--------+
	buf := []byte{SOCKS5Version, method}
	_, err := s.conn.Write(buf)
	return err
}
