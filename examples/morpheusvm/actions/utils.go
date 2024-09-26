package actions

import "os"

// LoadBytes returns bytes stored at a file [filename].
func LoadBytes(filename string) ([]byte, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
