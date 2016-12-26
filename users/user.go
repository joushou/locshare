package users

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	LoginRetryTimeLimit = time.Minute
	LoginRetryCount     = 3
	UserBufferLimit     = 64
)

type msgBox struct {
	source  string
	content []byte
}

func (m msgBox) Source() string {
	return m.source
}

func (m msgBox) Content() []byte {
	return m.content
}

type keyBox struct {
	id  uint64
	key []byte
}

type keyRespBox struct {
	username string
	id       uint64
	key      []byte
}

type user struct {
	username string

	passwordLock sync.RWMutex
	passwordHash []byte

	locationBufferLock sync.RWMutex
	locationBuffer     []msgBox

	subscriberLock sync.RWMutex
	subscribers    []chan UserMessage

	keyLock     sync.RWMutex
	keys        []keyBox
	signedKey   *keyBox
	identityKey []byte

	authLock     sync.RWMutex
	authFailCnt  int
	authFailTime time.Time
}

func (u *user) Username() string {
	return u.username
}

func (u *user) SetPassword(password string) error {
	u.passwordLock.Lock()
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		u.passwordLock.Unlock()
		return err
	}
	u.passwordHash = pass
	u.passwordLock.Unlock()
	return nil
}

func (u *user) authSuccess() error {
	u.authLock.Lock()
	u.authFailCnt = 0
	u.authLock.Unlock()
	return nil
}

func (u *user) authFail() error {
	n := time.Now()
	u.authLock.Lock()
	u.authFailTime = n
	u.authFailCnt++
	u.authLock.Unlock()

	return fmt.Errorf("unable to authenticate")
}

func (u *user) authRateLimit() error {
	u.authLock.RLock()
	authFailTime := u.authFailTime
	authFailCnt := u.authFailCnt
	u.authLock.RUnlock()

	n := time.Now()
	if n.Sub(authFailTime) < LoginRetryTimeLimit {
		if authFailCnt > LoginRetryCount {
			return fmt.Errorf("auth try limit reached; try again later")
		}
	}

	return nil
}

func (u *user) Authenticate(password string) error {
	if err := u.authRateLimit(); err != nil {
		return err
	}

	u.passwordLock.RLock()
	defer u.passwordLock.RUnlock()

	err := bcrypt.CompareHashAndPassword(u.passwordHash, []byte(password))
	if err != nil {
		return u.authFail()
	}
	return u.authSuccess()
}

func (u *user) SetIdentity(identity []byte) error {
	u.keyLock.Lock()
	defer u.keyLock.Unlock()
	u.identityKey = identity
	return nil
}

func (u *user) Identity() ([]byte, error) {
	u.keyLock.RLock()
	defer u.keyLock.RUnlock()
	if u.identityKey == nil {
		return nil, fmt.Errorf("user has no identity key")
	}
	return u.identityKey, nil
}

func (u *user) SetTemporaryKey(keyID uint64, key []byte) error {
	u.keyLock.Lock()
	defer u.keyLock.Unlock()
	u.signedKey = &keyBox{keyID, key}
	return nil
}

func (u *user) TemporaryKey() (uint64, []byte, error) {
	u.keyLock.RLock()
	defer u.keyLock.RUnlock()
	if u.signedKey == nil {
		return 0, nil, fmt.Errorf("user has no signed key")
	}
	k := u.signedKey
	return k.id, k.key, nil
}

func (u *user) SetOneTimeKey(keyID uint64, key []byte) error {
	u.keyLock.Lock()
	defer u.keyLock.Unlock()
	for _, k := range u.keys {
		if k.id == keyID {
			return fmt.Errorf("key ID already in use")
		}
	}

	u.keys = append(u.keys, keyBox{keyID, key})
	return nil
}

func (u *user) RemoveOneTimeKey(keyID uint64) error {
	u.keyLock.Lock()
	defer u.keyLock.Unlock()
	for idx, k := range u.keys {
		if k.id == keyID {
			u.keys = append(u.keys[:idx], u.keys[idx+1:]...)
			return nil
		}
	}

	return fmt.Errorf("no such key")
}

func (u *user) PopOneTimeKey() (uint64, []byte, error) {
	u.keyLock.Lock()
	defer u.keyLock.Unlock()
	if len(u.keys) == 0 {
		return 0, nil, fmt.Errorf("no keys available")
	}

	key := u.keys[0]
	u.keys = u.keys[1:]
	return key.id, key.key, nil
}

func (u *user) OneTimeKeys() ([]uint64, error) {
	u.keyLock.Lock()
	defer u.keyLock.Unlock()
	arr := make([]uint64, len(u.keys))
	for idx, k := range u.keys {
		arr[idx] = k.id
	}

	return arr, nil
}

func (u *user) pushToLocationBuffer(m msgBox) {
	u.locationBufferLock.Lock()
	defer u.locationBufferLock.Unlock()

	for i, mb := range u.locationBuffer {
		if mb.source == m.source {
			u.locationBuffer = append(u.locationBuffer[:i], u.locationBuffer[i+1:]...)
			break
		}
	}

	if len(u.locationBuffer) >= UserBufferLimit {
		limit := len(u.locationBuffer) - UserBufferLimit
		u.locationBuffer = u.locationBuffer[limit+1:]
	}

	u.locationBuffer = append(u.locationBuffer, m)
}

func (u *user) Publish(source string, content []byte) error {
	m := msgBox{source, content}
	u.subscriberLock.RLock()
	defer u.subscriberLock.RUnlock()
	if len(u.subscribers) == 0 {
		u.pushToLocationBuffer(m)
		return nil
	}

	for _, subscriber := range u.subscribers {
		subscriber <- m
	}

	return nil
}

func (u *user) Subscribe() (<-chan UserMessage, error) {
	u.locationBufferLock.RLock()
	defer u.locationBufferLock.RUnlock()

	ch := make(chan UserMessage, len(u.locationBuffer)*2)

	u.subscriberLock.Lock()
	u.subscribers = append(u.subscribers, ch)
	u.subscriberLock.Unlock()

	for _, m := range u.locationBuffer {
		ch <- m
	}

	u.locationBuffer = nil

	return ch, nil
}

func (u *user) Unsubscribe(ch <-chan UserMessage) error {
	u.subscriberLock.Lock()
	defer u.subscriberLock.Unlock()
	for i := range u.subscribers {
		if u.subscribers[i] == ch {
			close(u.subscribers[i])
			u.subscribers = append(u.subscribers[:i], u.subscribers[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("no such subcription")
}

func newUser(username, password string) (*user, error) {
	u := &user{username: username}
	return u, u.SetPassword(password)
}
