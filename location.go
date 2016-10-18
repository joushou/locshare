package locshare

import (
	"bytes"
	"encoding/binary"
	"errors"
)

type Location struct {
	Time      int64
	Accuracy  float64
	Latitude  float64
	Longitude float64
	Altitude  float64
	Bearing   float64
	Speed     float64
}

func Encode(loc Location) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, &loc.Time)
	binary.Write(buf, binary.BigEndian, &loc.Accuracy)
	binary.Write(buf, binary.BigEndian, &loc.Latitude)
	binary.Write(buf, binary.BigEndian, &loc.Longitude)
	binary.Write(buf, binary.BigEndian, &loc.Altitude)
	binary.Write(buf, binary.BigEndian, &loc.Bearing)
	binary.Write(buf, binary.BigEndian, &loc.Speed)

	return buf.Bytes()
}

func Decode(b []byte) (Location, error) {
	if len(b) != 8*7 {
		return Location{}, errors.New("buffer too short")
	}

	br := bytes.NewReader(b)

	var loc Location
	binary.Read(br, binary.BigEndian, &loc.Time)
	binary.Read(br, binary.BigEndian, &loc.Accuracy)
	binary.Read(br, binary.BigEndian, &loc.Latitude)
	binary.Read(br, binary.BigEndian, &loc.Longitude)
	binary.Read(br, binary.BigEndian, &loc.Altitude)
	binary.Read(br, binary.BigEndian, &loc.Bearing)
	binary.Read(br, binary.BigEndian, &loc.Speed)

	return loc, nil
}
