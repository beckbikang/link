package link

import (
	"sync"
)

//任意类型
type KEY interface{}

//chanel的状态
type Channel struct {
	mutex    sync.RWMutex
	sessions map[KEY]*Session

	// channel state
	State interface{}
}

//新建一个channel
func NewChannel() *Channel {
	return &Channel{
		sessions: make(map[KEY]*Session),
	}
}

//定义channel的长度
func (channel *Channel) Len() int {
	channel.mutex.RLock()
	defer channel.mutex.RUnlock()
	return len(channel.sessions)
}

//回调函数处理session，回调处理session
func (channel *Channel) Fetch(callback func(*Session)) {
	channel.mutex.RLock()
	defer channel.mutex.RUnlock()
	for _, session := range channel.sessions {
		callback(session)
	}
}

//通过key获取session
func (channel *Channel) Get(key KEY) *Session {
	channel.mutex.RLock()
	defer channel.mutex.RUnlock()
	session, _ := channel.sessions[key]
	return session
}

//加入
func (channel *Channel) Put(key KEY, session *Session) {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()
	if session, exists := channel.sessions[key]; exists {
		channel.remove(key, session)
	}
	//加入session的回调,在关闭session的时候关闭session
	session.AddCloseCallback(channel, key, func() {
		channel.Remove(key)
	})
	//设置新的session
	channel.sessions[key] = session
}

//删除一个session
func (channel *Channel) remove(key KEY, session *Session) {
	session.RemoveCloseCallback(channel, key)
	delete(channel.sessions, key)
}

//删除一个session对外
func (channel *Channel) Remove(key KEY) bool {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()
	session, exists := channel.sessions[key]
	if exists {
		channel.remove(key, session)
	}
	return exists
}

//删除session，同时删除回调函数，调用回调函数
func (channel *Channel) FetchAndRemove(callback func(*Session)) {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()
	for key, session := range channel.sessions {
		session.RemoveCloseCallback(channel, key)
		delete(channel.sessions, key)
		callback(session)
	}
}

//关闭channel
func (channel *Channel) Close() {
	channel.mutex.Lock()
	defer channel.mutex.Unlock()
	for key, session := range channel.sessions {
		channel.remove(key, session)
	}
}
