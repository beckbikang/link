package link

import (
	"errors"
	"sync"
	"sync/atomic"
)

//定义各种session错误
var SessionClosedError = errors.New("Session Closed")
var SessionBlockedError = errors.New("Session Blocked")

//sessionId
var globalSessionId uint64

type Session struct {
	id        uint64  //id
	codec     Codec   //解析器
	manager   *Manager //管理器
	sendChan  chan interface{}//发送通道
	recvMutex sync.Mutex //接收的锁
	sendMutex sync.RWMutex//发送的锁

	closeFlag          int32 //关闭flag
	closeChan          chan int//关闭的chan
	closeMutex         sync.Mutex//关闭的锁
	firstCloseCallback *closeCallback//初次关闭的回调
	lastCloseCallback  *closeCallback//最后关闭的回调

	State interface{}//状态
}

func NewSession(codec Codec, sendChanSize int) *Session {
	return newSession(nil, codec, sendChanSize)
}

func newSession(manager *Manager, codec Codec, sendChanSize int) *Session {
	//新建一个session
	session := &Session{
		codec:     codec,
		manager:   manager,
		closeChan: make(chan int),
		id:        atomic.AddUint64(&globalSessionId, 1),
	}

	//建立异步发送通道
	if sendChanSize > 0 {
		session.sendChan = make(chan interface{}, sendChanSize)
		go session.sendLoop()
	}
	return session
}

//当前id
func (session *Session) ID() uint64 {
	return session.id
}

//是否关闭
func (session *Session) IsClosed() bool {
	return atomic.LoadInt32(&session.closeFlag) == 1
}

//关闭
func (session *Session) Close() error {
	if atomic.CompareAndSwapInt32(&session.closeFlag, 0, 1) {
		//关闭closeChan
		close(session.closeChan)

		//发送chan不为空
		if session.sendChan != nil {
			session.sendMutex.Lock()
			//关闭发送chan
			close(session.sendChan)

			//实现ClearSendChan，就关闭它
			if clear, ok := session.codec.(ClearSendChan); ok {
				clear.ClearSendChan(session.sendChan)
			}
			session.sendMutex.Unlock()
		}
		//关闭解析器
		err := session.codec.Close()

		go func() {
			//关闭回调函数
			session.invokeCloseCallbacks()

			//从mannager里面关闭它
			if session.manager != nil {
				session.manager.delSession(session)
			}
		}()
		return err
	}

	//已经关闭的error
	return SessionClosedError
}

//返回值
func (session *Session) Codec() Codec {
	return session.codec
}

//接收数据
func (session *Session) Receive() (interface{}, error) {
	session.recvMutex.Lock()
	defer session.recvMutex.Unlock()

	msg, err := session.codec.Receive()
	if err != nil {
		session.Close()
	}
	return msg, err
}

//发送的loop
func (session *Session) sendLoop() {
	defer session.Close()
	for {
		select {
		case msg, ok := <-session.sendChan://接收send chan
			if !ok || session.codec.Send(msg) != nil {
				return
			}
		case <-session.closeChan://关闭chan
			return
		}
	}
}

//发送数据
func (session *Session) Send(msg interface{}) error {
	if session.sendChan == nil {
		//关闭
		if session.IsClosed() {
			return SessionClosedError
		}

		session.sendMutex.Lock()
		defer session.sendMutex.Unlock()
		//发送数据
		err := session.codec.Send(msg)
		if err != nil {
			session.Close()
		}
		//返回错误
		return err
	}

	//关闭错误
	session.sendMutex.RLock()
	if session.IsClosed() {
		session.sendMutex.RUnlock()
		return SessionClosedError
	}

	//消息发送的chan
	select {
	case session.sendChan <- msg:
		session.sendMutex.RUnlock()
		return nil
	default://阻塞错误
		session.sendMutex.RUnlock()
		session.Close()
		return SessionBlockedError
	}
}

type closeCallback struct {
	Handler interface{}//处理器
	Key     interface{}//key
	Func    func()//函数
	Next    *closeCallback//下一个处理器
}

func (session *Session) AddCloseCallback(handler, key interface{}, callback func()) {
	//关闭
	if session.IsClosed() {
		return
	}

	//关闭锁
	session.closeMutex.Lock()
	defer session.closeMutex.Unlock()

	//添加一个回调函数
	newItem := &closeCallback{handler, key, callback, nil}

	//加入回调链表
	if session.firstCloseCallback == nil {
		session.firstCloseCallback = newItem
	} else {
		session.lastCloseCallback.Next = newItem
	}
	session.lastCloseCallback = newItem
}

//删除处理器
func (session *Session) RemoveCloseCallback(handler, key interface{}) {
	if session.IsClosed() {
		return
	}

	session.closeMutex.Lock()
	defer session.closeMutex.Unlock()

	var prev *closeCallback
	for callback := session.firstCloseCallback; callback != nil; prev, callback = callback, callback.Next {
		if callback.Handler == handler && callback.Key == key {
			if session.firstCloseCallback == callback {
				session.firstCloseCallback = callback.Next
			} else {
				prev.Next = callback.Next
			}
			if session.lastCloseCallback == callback {
				session.lastCloseCallback = prev
			}
			return
		}
	}
}

//调用回调函数
func (session *Session) invokeCloseCallbacks() {
	session.closeMutex.Lock()
	defer session.closeMutex.Unlock()

	for callback := session.firstCloseCallback; callback != nil; callback = callback.Next {
		callback.Func()
	}
}
