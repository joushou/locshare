package users

import "sync"

type userDB struct {
	userLock sync.RWMutex
	users    map[string]User
}

func (db *userDB) Get(username string) (User, error) {
	db.userLock.RLock()
	u := db.users[username]
	db.userLock.RUnlock()
	if u == nil {
		return nil, ErrNoSuchUser
	}

	return u, nil
}

func (db *userDB) New(username, password string) (User, error) {
	db.userLock.Lock()
	defer db.userLock.Unlock()
	if db.users[username] != nil {
		return nil, ErrUserAlreadyExists
	}

	u, err := newUser(username, password)
	if err != nil {
		return nil, err
	}
	db.users[username] = u
	return u, nil
}

func (db *userDB) Del(username string) error {
	db.userLock.Lock()
	if db.users[username] == nil {
		db.userLock.Unlock()
		return ErrNoSuchUser
	}

	delete(db.users, username)
	db.userLock.Unlock()
	return nil
}

func NewDB() UserDB {
	return &userDB{
		users: make(map[string]User),
	}
}
