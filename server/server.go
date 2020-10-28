package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/kennylevinsen/locshare/mux"
	"github.com/kennylevinsen/locshare/sessions"
	"github.com/kennylevinsen/locshare/users"

	"github.com/gorilla/websocket"
)

const (
	PermissionDenied = http.StatusUnauthorized
	InvalidRequest   = http.StatusBadRequest
	ProcessingError  = http.StatusInternalServerError
	NoSuchEntity     = http.StatusNotFound
)

var (
	contextKeySession    = "session"
	contextKeyUserParam  = "user"
	contextKeyKeyIDParam = "keyid"
)

var upgrader = websocket.Upgrader{}

type errorResp struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func sendError(w http.ResponseWriter, code int, format string, v ...interface{}) {
	log.Printf("error: %s", fmt.Sprintf(format, v...))
	e := errorResp{
		Status: "error",
		Error:  fmt.Sprintf(format, v...),
	}

	b, err := json.Marshal(&e)
	if err != nil {
		w.WriteHeader(ProcessingError)
		return
	}
	w.WriteHeader(code)
	w.Write(b)
}

type Server struct {
	http.Handler
	sessions sessions.SessionDB
	users    users.UserDB
}

func (s *Server) requireValidToken(capability string, f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var token string
		tokenHdr := r.Header.Get("Authorization")
		if tokenHdr != "" {
			tokenParts := strings.Split(tokenHdr, " ")
			if len(tokenParts) == 2 && tokenParts[0] == "LOCSHARE" {
				token = tokenParts[1]
			}
		}

		session, err := s.sessions.Get(token)
		if err != nil {
			sendError(w, PermissionDenied, "no such session")
			return
		}

		if err := session.HasCapability(capability); err != nil {
			sendError(w, PermissionDenied, "access denied: %v", err)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeySession, session)
		f(w, r.WithContext(ctx))
	}
}

type authReq struct {
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	Capabilities []string `json:"capabilities"`
}

func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	var req authReq
	if err = json.Unmarshal(b, &req); err != nil {
		sendError(w, InvalidRequest, "could not parse request: %v", err)
		return
	}

	if req.Username == "" || req.Password == "" {
		sendError(w, InvalidRequest, "username and password must not be empty")
		return
	}

	if len(req.Capabilities) == 0 {
		sendError(w, InvalidRequest, "wanted capabilities must be specified")
		return
	}

	user, err := s.users.Get(req.Username)
	if err != nil {
		sendError(w, PermissionDenied, "unable to authenticate")
		return
	}

	if err = user.Authenticate(req.Password); err != nil {
		sendError(w, PermissionDenied, "unable to authenticate")
		return
	}

	session, err := s.sessions.New(req.Capabilities)
	if err != nil {
		sendError(w, ProcessingError, "unable to create session: %v", err)
		return
	}

	if err = session.SetUsername(req.Username); err != nil {
		sendError(w, ProcessingError, "unable to set up session: %v", err)
		return
	}

	w.Write([]byte(session.Token()))
}

type postUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) postUser(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	var req postUserReq
	if err = json.Unmarshal(b, &req); err != nil {
		sendError(w, InvalidRequest, "could not parse request: %v", err)
		return
	}

	if req.Username == "" || req.Password == "" {
		sendError(w, InvalidRequest, "username and password must not be empty")
		return
	}

	if _, err = s.users.New(req.Username, req.Password); err != nil {
		sendError(w, InvalidRequest, "could not create user: %v", err)
		return
	}

	w.Write([]byte("ok"))
}

type pastPasswordReq struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (s *Server) postPassword(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	var req pastPasswordReq
	if err = json.Unmarshal(b, &req); err != nil {
		sendError(w, InvalidRequest, "could not parse request: %v", err)
		return
	}

	if len(req.NewPassword) == 0 {
		sendError(w, InvalidRequest, "new password must not be empty")
		return
	}

	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	if err := user.Authenticate(req.OldPassword); err != nil {
		sendError(w, InvalidRequest, "invalid password")
		return
	}

	if err := user.SetPassword(req.NewPassword); err != nil {
		sendError(w, ProcessingError, "unable to set password")
		return
	}

	w.Write([]byte("ok"))
}

func (s *Server) getIdentity(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	identity, err := user.Identity()
	if err != nil {
		sendError(w, ProcessingError, "unable to retrieve identity: %v", err)
		return
	}

	w.Write(identity)
}

func (s *Server) putIdentity(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	if err := user.SetIdentity(b); err != nil {
		sendError(w, ProcessingError, "unable to set identity: %v", err)
		return
	}

	w.Write([]byte("ok"))
}

type getTemporaryKeyResp struct {
	KeyID uint64 `json:"keyID"`
	Key   []byte `json:"key"`
}

func (s *Server) getTemporaryKey(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	keyID, key, err := user.TemporaryKey()
	if err != nil {
		sendError(w, ProcessingError, "unable to retrieve signed key: %v", err)
		return
	}

	resp := getTemporaryKeyResp{
		KeyID: keyID,
		Key:   key,
	}

	b, err := json.Marshal(&resp)
	if err != nil {
		sendError(w, ProcessingError, "unable to marshal response: %v", err)
		return
	}

	w.Write(b)
}

func (s *Server) putTemporaryKey(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	keyID, err := strconv.ParseUint(r.Context().Value(contextKeyKeyIDParam).(string), 10, 64)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	if err := user.SetTemporaryKey(keyID, b); err != nil {
		sendError(w, ProcessingError, "unable to set signed key: %v", err)
		return
	}

	w.Write([]byte("ok"))
}

func (s *Server) putOneTimeKey(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	keyID, err := strconv.ParseUint(r.Context().Value(contextKeyKeyIDParam).(string), 10, 64)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	if err := user.SetOneTimeKey(keyID, b); err != nil {
		sendError(w, ProcessingError, "unable to set key: %v", err)
		return
	}

	w.Write([]byte("ok"))
}

func (s *Server) deleteOneTimeKey(w http.ResponseWriter, r *http.Request) {
	keyID, err := strconv.ParseUint(r.Context().Value(contextKeyKeyIDParam).(string), 10, 64)
	if err != nil {
		sendError(w, NoSuchEntity, "parameter not uint: %v", err)
		return
	}

	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	if err := user.RemoveOneTimeKey(keyID); err != nil {
		sendError(w, ProcessingError, "unable to delete key: %v", err)
		return
	}

	w.Write([]byte("ok"))
}

type getOneTimeKeysResp struct {
	Keys []uint64 `json:"keys"`
}

func (s *Server) getOneTimeKeys(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	keys, err := user.OneTimeKeys()
	if err != nil {
		sendError(w, ProcessingError, "unable to retrieve keys: %v", err)
		return
	}

	resp := getOneTimeKeysResp{
		Keys: keys,
	}

	b, err := json.Marshal(&resp)
	if err != nil {
		sendError(w, ProcessingError, "unable to marshal response: %v", err)
		return
	}

	w.Write(b)
}

type getOneTimeKeyResp struct {
	KeyID uint64 `json:"keyID"`
	Key   []byte `json:"key"`
}

func (s *Server) getOneTimeKey(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	keyID, key, err := user.PopOneTimeKey()
	if err != nil {
		sendError(w, ProcessingError, "unable to retrieve one time key: %v", err)
		return
	}

	resp := getOneTimeKeyResp{
		KeyID: keyID,
		Key:   key,
	}

	b, err := json.Marshal(&resp)
	if err != nil {
		sendError(w, ProcessingError, "unable to marshal response: %v", err)
		return
	}

	w.Write(b)
}

func (s *Server) putMessage(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		sendError(w, InvalidRequest, "could not read body: %v", err)
		return
	}

	sess := r.Context().Value(contextKeySession).(sessions.Session)
	source, err := sess.Username()
	if err != nil {
		sendError(w, ProcessingError, "unable to retrieve username from session: %v", err)
		return
	}

	username := r.Context().Value(contextKeyUserParam).(string)
	user, err := s.users.Get(username)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	log.Printf("%s -> %v", source, b)

	if err := user.Publish(source, b); err != nil {
		sendError(w, ProcessingError, "publish failed: %v", err)
		return
	}

	w.Write([]byte("ok"))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(contextKeyUserParam).(string)
	if err := s.users.Del(username); err != nil {
		sendError(w, ProcessingError, "unable to delete user")
		return
	}

	w.Write([]byte("ok"))
}

type subscribeResp struct {
	Source  string `json:"source"`
	Content []byte `json:"content"`
}

func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) {
	sess := r.Context().Value(contextKeySession).(sessions.Session)
	source, err := sess.Username()
	if err != nil {
		sendError(w, ProcessingError, "unable to retrieve username from session: %v", err)
		return
	}

	user, err := s.users.Get(source)
	if err != nil {
		sendError(w, NoSuchEntity, "unable to retrieve user: %v", err)
		return
	}

	ch, err := user.Subscribe()
	if err != nil {
		sendError(w, ProcessingError, "unable to subscribe: %v", err)
		return
	}
	defer user.Unsubscribe(ch)

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	defer c.Close()

	for msg := range ch {
		if !sess.IsValid() {
			return
		}

		jsonMsg := subscribeResp{msg.Source(), msg.Content()}
		if err := c.WriteJSON(&jsonMsg); err != nil {
			return
		}
	}
}

func paramIsSelf(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := r.Context().Value(contextKeySession).(sessions.Session)
		username, err := sess.Username()
		if err != nil {
			sendError(w, ProcessingError, "unable to retrieve username from session: %v", err)
			return
		}

		userParam := r.Context().Value(contextKeyUserParam).(string)
		if username != userParam {
			sendError(w, PermissionDenied, "access denied")
			return
		}

		f(w, r)
	}
}

func (s *Server) setupMux() {
	interactive := func(h http.HandlerFunc) http.HandlerFunc {
		return s.requireValidToken("interactive", h)
	}
	publish := func(h http.HandlerFunc) http.HandlerFunc {
		return s.requireValidToken("publish", h)
	}
	destroyer := func(h http.HandlerFunc) http.HandlerFunc {
		return s.requireValidToken("destroyer", h)
	}
	w := func(f http.HandlerFunc, h ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
		for idx := len(h) - 1; idx >= 0; idx-- {
			f = h[idx](f)
		}

		return f
	}
	param := mux.NewParam
	method := mux.NewMethod
	s.Handler = mux.New().
		Handle("/auth", method().
			MethodFunc("POST", s.auth)).
		Handle("/user", param(contextKeyUserParam).
			Param(mux.New().
				Handle("/password", method().
					MethodFunc("POST", w(s.postPassword, interactive, paramIsSelf))).
				Handle("/identity", method().
					MethodFunc("GET", w(s.getIdentity, interactive)).
					MethodFunc("PUT", w(s.putIdentity, interactive, paramIsSelf))).
				Handle("/temporaryKey", param(contextKeyKeyIDParam).
					Param(method().
						MethodFunc("PUT", w(s.putTemporaryKey, interactive, paramIsSelf))).
					NoParam(method().
						MethodFunc("GET", w(s.getTemporaryKey, interactive)))).
				Handle("/oneTimeKey", param(contextKeyKeyIDParam).
					Param(method().
						MethodFunc("PUT", w(s.putOneTimeKey, interactive, paramIsSelf)).
						MethodFunc("DELETE", w(s.deleteOneTimeKey, interactive, paramIsSelf))).
					NoParam(method().
						MethodFunc("GET", w(s.getOneTimeKey, interactive)))).
				Handle("/oneTimekeys", method().
					MethodFunc("GET", w(s.getOneTimeKeys, interactive, paramIsSelf))).
				Handle("/message", method().
					MethodFunc("PUT", w(s.putMessage, publish))).
				Handle("/", method().
					MethodFunc("DELETE", w(s.deleteUser, destroyer, paramIsSelf)))).
			NoParam(method().
				MethodFunc("POST", s.postUser))).
		Handle("/ws", mux.New().
			Handle("/subscribe", w(s.subscribe, interactive))).
		Otherwise(http.FileServer(http.Dir(".")))

	s.Handler = mux.NewLogger(s.Handler)
}

func NewServer() *Server {
	s := Server{
		sessions: sessions.NewDB(),
		users:    users.NewDB(),
	}

	s.setupMux()

	return &s
}
