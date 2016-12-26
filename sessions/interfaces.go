package sessions

import "errors"

var (
	ErrNoSuchSession    = errors.New("no such session")
	ErrNoSuchCapability = errors.New("no such capability")
)

type Session interface {
	SetUsername(username string) error
	Username() (string, error)

	HasCapability(capability string) error
	IsValid() bool
	Invalidate() error

	Token() string
}

type SessionDB interface {
	New(capabilities []string) (Session, error)
	Get(token string) (Session, error)
	Del(token string) error
}
