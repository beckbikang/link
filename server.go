package link

import "net"

type Server struct {
	manager      *Manager //session管理器
	listener     net.Listener //网络连接
	protocol     Protocol //协议解析器
	handler      Handler//session处理器
	sendChanSize int//发送chan的大小
}

//处理接口
type Handler interface {
	HandleSession(*Session)
}

//让编译器检查，以确保某个类型实现接⼝口
var _ Handler = HandlerFunc(nil)

//函数实现接口
type HandlerFunc func(*Session)

func (f HandlerFunc) HandleSession(session *Session) {
	f(session)
}

func NewServer(listener net.Listener, protocol Protocol, sendChanSize int, handler Handler) *Server {
	return &Server{
		manager:      NewManager(),
		listener:     listener,
		protocol:     protocol,
		handler:      handler,
		sendChanSize: sendChanSize,
	}
}

//返回当前的listen
func (server *Server) Listener() net.Listener {
	return server.listener
}

func (server *Server) Serve() error {
	for {
		//获取当前请求的连接
		conn, err := Accept(server.listener)
		if err != nil {
			return err
		}

		go func() {
			//创建一个解析器对象
			codec, err := server.protocol.NewCodec(conn)

			//处理失败
			if err != nil {
				conn.Close()
				return
			}

			//通过manager新建一个session
			session := server.manager.NewSession(codec, server.sendChanSize)
			//处理session
			server.handler.HandleSession(session)
		}()
	}
}

func (server *Server) GetSession(sessionID uint64) *Session {
	return server.manager.GetSession(sessionID)
}

//关闭server  1 关闭连接 2 丢弃session
func (server *Server) Stop() {
	server.listener.Close()
	server.manager.Dispose()
}
