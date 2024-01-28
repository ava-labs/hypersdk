package archiver

import "encoding/base32"

func base32EncodeToString(k []byte) string {
	return base32.StdEncoding.EncodeToString(k)
}
