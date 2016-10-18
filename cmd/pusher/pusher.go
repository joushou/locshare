package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/joushou/ecies"
	"github.com/joushou/locshare"
)

func main() {
	in, err := base64.StdEncoding.DecodeString(os.Args[2])
	if err != nil {
		fmt.Printf("invalid key: %v\n", err)
		return
	}

	loc := locshare.Location{
		Time: time.Now().UTC().Unix() * 1000,
	}

	loc.Accuracy, _ = strconv.ParseFloat(os.Args[3], 64)
	loc.Latitude, _ = strconv.ParseFloat(os.Args[4], 64)
	loc.Longitude, _ = strconv.ParseFloat(os.Args[5], 64)
	loc.Altitude, _ = strconv.ParseFloat(os.Args[6], 64)
	loc.Bearing, _ = strconv.ParseFloat(os.Args[7], 64)
	loc.Speed, _ = strconv.ParseFloat(os.Args[8], 64)

	buf := locshare.Encode(loc)

	c, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		fmt.Printf("error dialing service: %v\n", err)
		return
	}

	res, err := ecies.Encrypt(buf, in)

	fmt.Fprintf(c, "pub %s %s\n", os.Args[2], base64.StdEncoding.EncodeToString(res))

	c.Close()

}
