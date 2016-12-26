package proto

import (
	"bytes"
	"encoding/binary"
	"io"
)

func ProtoWrite(data map[string]string, c io.Writer) error {
	b := new(bytes.Buffer)
	b.Write([]byte{0, 0}) // dummy length

	for k, v := range data {
		b.Write([]byte{byte(len(k))})
		b.Write([]byte(k))
		b.Write([]byte{byte(len(v))})
		b.Write([]byte(v))
	}

	bytes := b.Bytes()
	binary.BigEndian.PutUint16(bytes, uint16(len(bytes)-2))

	_, err := c.Write(bytes)
	return err
}

func ProtoRead(c io.Reader) (map[string]string, error) {
	header := make([]byte, 2)
	param_header := make([]byte, 1)

	_, err := io.ReadFull(c, header)
	if err != nil {
		return nil, err
	}

	l := binary.BigEndian.Uint16(header)

	data := make(map[string]string)

	for l > 0 {
		if _, err := io.ReadFull(c, param_header); err != nil {
			return nil, err
		}

		key_len := uint8(param_header[0])
		key := make([]byte, key_len)

		if _, err := io.ReadFull(c, key); err != nil {
			return nil, err
		}

		if _, err := io.ReadFull(c, param_header); err != nil {
			return nil, err
		}

		value_len := uint8(param_header[0])
		value := make([]byte, value_len)

		if _, err := io.ReadFull(c, value); err != nil {
			return nil, err
		}

		l -= uint16(value_len + key_len + 2)

		data[string(key)] = string(value)
	}

	return data, nil
}
