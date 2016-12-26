package main

import (
	"net/http"

	"github.com/joushou/locshare/server"
)

func main() {
	s := server.NewServer()
	http.ListenAndServe(":9000", s)
}
