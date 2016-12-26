package sessions

import "sync"

type session struct {
	token        string
	capabilities []string

	validLock sync.RWMutex
	valid     bool

	usernameLock sync.RWMutex
	username     string
}

func (s *session) HasCapability(capability string) error {
	for _, c := range s.capabilities {
		if c == capability {
			return nil
		}
	}

	return ErrNoSuchCapability
}

func (s *session) IsValid() bool {
	s.validLock.RLock()
	defer s.validLock.RUnlock()
	return s.valid
}

func (s *session) Invalidate() error {
	s.validLock.Lock()
	s.valid = false
	s.validLock.Unlock()
	return nil
}

func (s *session) Token() string {
	return s.token
}

func (s *session) Username() (string, error) {
	s.usernameLock.RLock()
	defer s.usernameLock.RUnlock()
	return s.username, nil
}

func (s *session) SetUsername(username string) error {
	s.usernameLock.Lock()
	defer s.usernameLock.Unlock()
	s.username = username
	return nil
}

func newSession(token string, capabilities []string) Session {
	return &session{token: token, capabilities: capabilities, valid: true}
}
