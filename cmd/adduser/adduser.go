package main

import (
	"fmt"
	"os"

	"github.com/kennylevinsen/locshare/client"
)

func main() {
	c := client.New(os.Args[1])

	err := c.NewUser(os.Args[2], os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failure: %v\n", err)
		return
	}
}
