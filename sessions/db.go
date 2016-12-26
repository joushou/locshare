package sessions

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
)

type sessionDB struct {
	sessionLock sync.RWMutex
	sessions    map[string]Session
}

func (db *sessionDB) New(capabilities []string) (Session, error) {
	db.sessionLock.Lock()
	session := ""
	for i := 0; i < 10; i++ {
		b := make([]byte, 64)
		_, err := rand.Read(b)
		if err != nil {
			continue
		}
		t := base64.URLEncoding.EncodeToString(b)

		if db.sessions[t] == nil {
			session = t
			break
		}
	}

	if session == "" {
		db.sessionLock.Unlock()
		return nil, errors.New("unable to create session")
	}

	s := newSession(session, capabilities)
	db.sessions[session] = s
	db.sessionLock.Unlock()
	return s, nil
}

func (db *sessionDB) Get(session string) (Session, error) {
	db.sessionLock.RLock()
	s := db.sessions[session]
	db.sessionLock.RUnlock()
	if s == nil {
		return nil, ErrNoSuchSession
	}

	return s, nil
}

func (db *sessionDB) Del(session string) error {
	db.sessionLock.Lock()
	sess := db.sessions[session]
	if sess == nil {
		db.sessionLock.Unlock()
		return ErrNoSuchSession
	}
	sess.Invalidate()
	delete(db.sessions, session)
	db.sessionLock.Unlock()
	return nil
}

func NewDB() SessionDB {
	return &sessionDB{
		sessions: make(map[string]Session),
	}
}
