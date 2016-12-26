package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (h *HTTPError) Error() string {
	return fmt.Sprintf("http error: %d: %s: %s", h.StatusCode, h.Status, h.Body)
}

type Client struct {
	Address string
	token   string
}

func New(addr string) *Client {
	return &Client{
		Address: addr,
	}
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	if c.token != "" {
		req.Header.Set("Cookie", "token="+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, &HTTPError{resp.StatusCode, resp.Status, string(b)}
	}

	return b, nil
}

func (c *Client) get(urlPath string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.Address+urlPath, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) getJSON(urlPath string, resp interface{}) error {
	b, err := c.get(urlPath)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	return json.Unmarshal(b, resp)
}

func (c *Client) post(urlPath string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest("POST", c.Address+urlPath, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest("POST", c.Address+urlPath, nil)
		if err != nil {
			return nil, err
		}
	}
	return c.do(req)
}

func (c *Client) postJSON(urlPath string, body, resp interface{}) error {
	var b []byte
	var err error
	if body != nil {
		if b, err = json.Marshal(body); err != nil {
			return err
		}
	}

	b, err = c.post(urlPath, b)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}

	return json.Unmarshal(b, resp)
}

func (c *Client) put(urlPath string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest("PUT", c.Address+urlPath, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest("PUT", c.Address+urlPath, nil)
		if err != nil {
			return nil, err
		}
	}
	return c.do(req)
}

func (c *Client) putJSON(urlPath string, body, resp interface{}) error {
	var b []byte
	var err error
	if body != nil {
		if b, err = json.Marshal(body); err != nil {
			return err
		}
	}

	b, err = c.put(urlPath, b)
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	return json.Unmarshal(b, resp)
}

func (c *Client) delete(urlPath string) ([]byte, error) {
	req, err := http.NewRequest("DELETE", c.Address+urlPath, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) deleteJSON(urlPath string, resp interface{}) error {
	b, err := c.delete(urlPath)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}
	return json.Unmarshal(b, resp)
}

type loginReq struct {
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	Capabilities []string `json:"capabilities"`
}

func (c *Client) Login(username, password string, capabilities []string) error {
	req := loginReq{username, password, capabilities}
	b, err := json.Marshal(&req)
	if err != nil {
		return err
	}
	b, err = c.post("/auth", b)
	if err != nil {
		return err
	}

	if len(b) == 0 {
		return errors.New("token length 0")
	}

	c.token = string(b)
	return nil
}

type newUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c *Client) NewUser(username, password string) error {
	req := newUserReq{username, password}
	return c.putJSON("/user", &req, nil)
}

func (c *Client) DeleteUser(username, password string) error {
	_, err := c.delete("/user/" + username + "/name")
	return err
}

func (c *Client) Name(username string) (string, error) {
	b, err := c.get("/user/" + username + "/name")
	return string(b), err
}

func (c *Client) SetName(username, name string) error {
	_, err := c.put("/user/"+username+"/name", []byte(name))
	return err
}

type postPasswordReq struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (c *Client) SetPassword(username, oldpassword, newpassword string) error {
	req := postPasswordReq{oldpassword, newpassword}
	return c.postJSON("/user/"+username+"/password", &req, nil)
}

func (c *Client) Identity(username string) ([]byte, error) {
	return c.get("/user/" + username + "/identity")
}

func (c *Client) SetIdentity(username string, identity []byte) error {
	_, err := c.put("/user/"+username+"/identity", identity)
	return err
}

type getSignedKeyResp struct {
	KeyID uint64 `json:"keyID"`
	Key   []byte `json:"key"`
}

func (c *Client) SignedKey(username string) (uint64, []byte, error) {
	var r getSignedKeyResp
	err := c.getJSON("/user/"+username+"/signedKey", &r)
	return r.KeyID, r.Key, err
}

func (c *Client) SetSignedKey(username string, keyID uint64, key []byte) error {
	_, err := c.put(fmt.Sprintf("/user/%s/signedKey/%d", username, keyID), key)
	return err
}

func (c *Client) SetKey(username string, keyID uint64, key []byte) error {
	_, err := c.put(fmt.Sprintf("/user/%s/key/%d", username, keyID), key)
	return err
}

func (c *Client) DeleteKey(username string, keyID uint64) error {
	_, err := c.delete(fmt.Sprintf("/user/%s/key/%d", username, keyID))
	return err
}

type getKeyResp struct {
	KeyID uint64 `json:"keyID"`
	Key   []byte `json:"key"`
}

func (c *Client) Key(username string) (uint64, []byte, error) {
	var r getKeyResp
	err := c.getJSON("/user/"+username+"/key", &r)
	return r.KeyID, r.Key, err
}

type getKeysResp struct {
	Keys []uint64 `json:"keys"`
}

func (c *Client) Keys(username string) ([]uint64, error) {
	var r getKeysResp
	err := c.getJSON("/user/"+username+"/keys", &r)
	return r.Keys, err
}

func (c *Client) DeleteMessages(username string) error {
	_, err := c.delete("/user/" + username + "/messages")
	return err
}

func (c *Client) SendMessage(username string, content []byte) error {
	_, err := c.put("/user/"+username+"/message", content)
	return err
}
