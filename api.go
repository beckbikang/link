package link

import (
	"io"
	"net"
	"strings"
	"time"
)

//定义协议接口
type Protocol interface {
	NewCodec(rw io.ReadWriter) (Codec, error)
}

// 协议接口的实现函数
type ProtocolFunc func(rw io.ReadWriter) (Codec, error)
func (pf ProtocolFunc) NewCodec(rw io.ReadWriter) (Codec, error) {
	return pf(rw)
}

//解析器接口 1 接收 2 发送 3 关闭
type Codec interface {
	Receive() (interface{}, error)
	Send(interface{}) error
	Close() error
}

//清理关闭chan的接口
type ClearSendChan interface {
	ClearSendChan(<-chan interface{})
}

//监听网络请求,返回一个server
func Listen(network, address string, protocol Protocol, sendChanSize int, handler Handler) (*Server, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return NewServer(listener, protocol, sendChanSize, handler), nil
}

//返回一个session
func Dial(network, address string, protocol Protocol, sendChanSize int) (*Session, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	codec, err := protocol.NewCodec(conn)
	if err != nil {
		return nil, err
	}
	return NewSession(codec, sendChanSize), nil
}

//返回一个支持超时的dial
func DialTimeout(network, address string, timeout time.Duration, protocol Protocol, sendChanSize int) (*Session, error) {
	conn, err := net.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	codec, err := protocol.NewCodec(conn)
	if err != nil {
		return nil, err
	}
	return NewSession(codec, sendChanSize), nil
}

//监听网络请求
func Accept(listener net.Listener) (net.Conn, error) {
	var tempDelay time.Duration
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				return nil, io.EOF
			}
			return nil, err
		}
		return conn, nil
	}
}
