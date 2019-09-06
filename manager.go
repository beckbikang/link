package link

import "sync"

const sessionMapNum = 32

//session管理器
type Manager struct {
	sessionMaps [sessionMapNum]sessionMap //分片管理
	disposeOnce sync.Once //初始化一次
	disposeWait sync.WaitGroup //wait
}

type sessionMap struct {
	sync.RWMutex//读写锁
	sessions map[uint64]*Session //存储session
	disposed bool //是否丢弃
}

//初始化管理器
func NewManager() *Manager {
	manager := &Manager{}
	for i := 0; i < len(manager.sessionMaps); i++ {
		manager.sessionMaps[i].sessions = make(map[uint64]*Session)
	}
	return manager
}

//丢弃和关闭连接
func (manager *Manager) Dispose() {
	manager.disposeOnce.Do(func() {
		for i := 0; i < sessionMapNum; i++ {
			smap := &manager.sessionMaps[i]
			smap.Lock()
			smap.disposed = true
			//循环处理session
			for _, session := range smap.sessions {
				//关闭session
				session.Close()
			}
			smap.Unlock()
		}
		manager.disposeWait.Wait()
	})
}

//新建一个session
func (manager *Manager) NewSession(codec Codec, sendChanSize int) *Session {
	session := newSession(manager, codec, sendChanSize)
	manager.putSession(session)
	return session
}

//获取一个session，根据sessionID
func (manager *Manager) GetSession(sessionID uint64) *Session {
	smap := &manager.sessionMaps[sessionID%sessionMapNum]
	smap.RLock()
	defer smap.RUnlock()

	session, _ := smap.sessions[sessionID]
	return session
}

//放入一个session，同时wait加 1
func (manager *Manager) putSession(session *Session) {
	smap := &manager.sessionMaps[session.id%sessionMapNum]

	smap.Lock()
	defer smap.Unlock()

	if smap.disposed {
		session.Close()
		return
	}

	smap.sessions[session.id] = session
	manager.disposeWait.Add(1)
}
//删除一个session wait -1
func (manager *Manager) delSession(session *Session) {
	smap := &manager.sessionMaps[session.id%sessionMapNum]

	smap.Lock()
	defer smap.Unlock()

	if _, ok := smap.sessions[session.id]; ok {
		delete(smap.sessions, session.id)
		manager.disposeWait.Done()
	}
}
