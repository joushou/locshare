package main

import (
	"net/http"

	"github.com/kennylevinsen/locshare/server"
)

func main() {
	s := server.NewServer()
	http.ListenAndServe(":9000", s)
}
