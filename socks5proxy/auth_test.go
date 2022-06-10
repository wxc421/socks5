package socks5proxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

func TestNewClientAuthMessage(t *testing.T) {
	//b := []byte{SOCKS5Version, 0x02, 0x00, 0x01, 0x00}
	//reader := bytes.NewReader(b)
	//message, err := NewClientAuthMessage(reader)
	//fmt.Println(message)
	//fmt.Println(err)
}

func TestNewServerAuthMessage(t *testing.T) {
	//t.Run("should send noauth", func(t *testing.T) {
	//	var buf bytes.Buffer
	//	err := NewServerAuthMessage(&buf, MethodNoAuth)
	//	if err != nil {
	//		t.Fatalf("should get nil error but got %s", err)
	//	}
	//
	//	got := buf.Bytes()
	//	if !reflect.DeepEqual(got, []byte{SOCKS5Version, MethodNoAuth}) {
	//		t.Fatalf("should send %v, but send %v", []byte{SOCKS5Version, MethodNoAuth}, got)
	//	}
	//})
	//
	//t.Run("should send no acceptable", func(t *testing.T) {
	//	var buf bytes.Buffer
	//	err := NewServerAuthMessage(&buf, MethodNoAcceptable)
	//	if err != nil {
	//		t.Fatalf("should get nil error but got %s", err)
	//	}
	//
	//	got := buf.Bytes()
	//	if !reflect.DeepEqual(got, []byte{SOCKS5Version, MethodNoAcceptable}) {
	//		t.Fatalf("should send %v, but send %v", []byte{SOCKS5Version, MethodNoAcceptable}, got)
	//	}
	//})
}

func TestName(t *testing.T) {
	i := []byte{1, 2, 3, 4, 5}
	fmt.Println(i[0 : len(i)-2])
}
func TestName2(t *testing.T) {
	var i1 int64 = 511 // [00000000 00000000 ... 00000001 11111111] = [0 0 0 0 0 0 1 255]

	s1 := make([]byte, 0)
	buf := bytes.NewBuffer(s1)

	// 数字转 []byte, 网络字节序为大端字节序
	binary.Write(buf, binary.BigEndian, i1)
	fmt.Println(buf.Bytes())
}

func TestParseDomain(t *testing.T) {
	ptr, err := net.LookupAddr("202.182.123.26")
	if err != nil {
		fmt.Println(err)
	}
	for _, ptrvalue := range ptr {
		fmt.Println(ptrvalue)
	}
}
