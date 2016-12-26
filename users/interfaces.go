package users

import "errors"

var (
	ErrNoSuchUser        = errors.New("no such user")
	ErrUserAlreadyExists = errors.New("user already exists")
)

type UserDB interface {
	New(username, password string) (User, error)
	Get(username string) (User, error)
	Del(username string) error
}

type UserMessage interface {
	Source() string
	Content() []byte
}

type User interface {
	// User properties
	Username() string

	// Authentication
	SetPassword(newpw string) error
	Authenticate(password string) error

	// Identity key
	SetIdentity(identity []byte) error
	Identity() (identity []byte, err error)

	// Signed prekey
	SetTemporaryKey(keyID uint64, key []byte) error
	TemporaryKey() (keyID uint64, key []byte, err error)

	// Prekeys
	SetOneTimeKey(keyID uint64, key []byte) error
	RemoveOneTimeKey(keyID uint64) error
	PopOneTimeKey() (keyID uint64, key []byte, err error)
	OneTimeKeys() (keyIDs []uint64, err error)

	// Message management
	Publish(source string, content []byte) error
	Subscribe() (<-chan UserMessage, error)
	Unsubscribe(ch <-chan UserMessage) error
}
