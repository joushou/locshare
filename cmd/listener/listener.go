package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/joushou/ecies"
	"github.com/joushou/locshare"
)

func main() {

	privKey, err := base64.StdEncoding.DecodeString(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding private key: %v\n", err)
		return
	}

	if len(privKey) != 32 && len(privKey) != 33 {
		fmt.Fprintf(os.Stderr, "invalid private key\n")
		return
	}

	var priv [32]byte
	copy(priv[:], privKey[:])

	var pub [32]byte
	curve25519.ScalarBaseMult(&pub, &priv)

	pubstr := base64.StdEncoding.EncodeToString(pub[:])
	fmt.Fprintf(os.Stderr, "Listening on: %s\n", pubstr)

	c, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error dialing service: %v\n", err)
		return
	}

	_, err = fmt.Fprintf(c, "sub %s\n", pubstr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error subscribing to feed: %v\n", err)
		return
	}
	for {
		r := bufio.NewReader(c)
		s, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "unable to read string: %v\n", err)
			}
			return
		}
		s = s[:len(s)-1]
		cmds := strings.Split(s, " ")
		if len(cmds) != 2 {
			fmt.Fprintf(os.Stderr, "invalid message received: %s\n", s)
			continue
		}

		blob, err := base64.StdEncoding.DecodeString(cmds[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding data: %v\n", err)
			return
		}

		res, err := ecies.Decrypt(blob, priv[:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decrypting: %v\n", err)
			return
		}

		loc, err := locshare.Decode(res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding location: %v\n", err)
			return
		}

		fmt.Printf("%v: accuracy: %.2f, coordinates: %.5f, %.5f, altitude: %.2f, bearing: %.2f, speed: %.2f\n", time.Unix(int64(loc.Time)/1000, 0), loc.Accuracy, loc.Latitude, loc.Longitude, loc.Altitude, loc.Bearing, loc.Speed)
	}
}
