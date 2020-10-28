package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/curve25519"

	"github.com/kennylevinsen/ecies"
	"github.com/kennylevinsen/locshare"
)

func main() {

	privKey, err := base64.StdEncoding.DecodeString(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding private key: %v\n", err)
		return
	}

	if len(privKey) != 32 {
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

	req := map[string]string{
		"m":    "auth",
		"user": os.Args[3],
		"pass": os.Args[4],
	}

	if err := locshare.ProtoWrite(req, c); err != nil {
		fmt.Fprintf(os.Stderr, "error writing proto: %v\n", err)
		return
	}

	msg, err := locshare.ProtoRead(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading proto: %v\n", err)
		return
	}

	token := msg["t"]

	if token == "" {
		fmt.Fprintf(os.Stderr, "auth failed: %s\n", msg["error"])
		return
	}

	req = map[string]string{
		"m": "t",
		"user": os.Args[3],
		"t": token,
	}

	if err := locshare.ProtoWrite(req, c); err != nil {
		fmt.Fprintf(os.Stderr, "error writing proto: %v\n", err)
		return
	}

	msg, err = locshare.ProtoRead(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading proto: %v\n", err)
		return
	}

	if msg["m"] != "ok" {
		fmt.Fprintf(os.Stderr, "token auth failed: %s\n", msg["error"])
		return
	}

	req = map[string]string{
		"m": "sub",
	}

	if err := locshare.ProtoWrite(req, c); err != nil {
		fmt.Fprintf(os.Stderr, "error writing proto: %v\n", err)
		return
	}

	for {
		msg, err := locshare.ProtoRead(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading proto: %v\n", err)
			return
		}

		switch msg["m"] {
		case "error":
			fmt.Fprintf(os.Stderr, "error: %v\n", msg["error"])
			return
		case "p":
			if msg["o"] != pubstr {
				fmt.Printf("Got message from unknown sender %s, skipping\n", msg["o"])
				continue
			}

			blob := []byte(msg["l"])
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

		default:
			fmt.Fprintf(os.Stderr, "no method\n")
			return
		}
	}
}
