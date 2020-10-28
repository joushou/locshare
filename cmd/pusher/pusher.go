package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/kennylevinsen/ecies"
	"github.com/kennylevinsen/locshare"
	"github.com/kennylevinsen/locshare/client"
)

func main() {
	in, err := base64.StdEncoding.DecodeString(os.Args[5])
	if err != nil {
		fmt.Printf("invalid key: %v\n", err)
		return
	}

	loc := locshare.Location{
		Time: time.Now().UTC().Unix() * 1000,
	}

	loc.Accuracy, _ = strconv.ParseFloat(os.Args[6], 64)
	loc.Latitude, _ = strconv.ParseFloat(os.Args[7], 64)
	loc.Longitude, _ = strconv.ParseFloat(os.Args[8], 64)
	loc.Altitude, _ = strconv.ParseFloat(os.Args[9], 64)
	loc.Bearing, _ = strconv.ParseFloat(os.Args[10], 64)
	loc.Speed, _ = strconv.ParseFloat(os.Args[11], 64)

	buf := locshare.Encode(loc)

	res, err := ecies.Encrypt(buf, in)
	if err != nil {
		fmt.Printf("error encrypting thingie: %v\n", err)
		return
	}

	c := client.New(os.Args[1])
	if err := c.Login(os.Args[2], os.Args[3], []string{"interactive", "publish"}); err != nil {
		fmt.Printf("authentication failed: %v\n", err)
		return
	}
	if err := c.SendMessage(os.Args[4], res); err != nil {
		fmt.Printf("push failed: %v\n", err)
		return
	}
}
