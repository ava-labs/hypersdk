package actions

import (
	"os"

	"github.com/near/borsh-go"
)

// LoadBytes returns bytes stored at a file [filename].
func LoadBytes(filename string) ([]byte, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func SerializeArgs(args interface{}) ([]byte, error) {
	bytes, err := borsh.Serialize(args)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}