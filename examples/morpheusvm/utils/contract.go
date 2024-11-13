package contract

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

type ParsedABI struct {
	ABI              abi.ABI
	Bytecode         []byte
	DeployedBytecode []byte
}

type rawJSON string

func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = rawJSON(data)
	return nil
}

func (r rawJSON) AsString() string {
	return string(r[1 : len(r)-1])
}

func NewABI(compiledFn string) (*ParsedABI, error) {
	f, err := os.Open(compiledFn)
	if err != nil {
		return nil, err
	}

	mapData := make(map[string]rawJSON)
	if err := json.NewDecoder(f).Decode(&mapData); err != nil {
		return nil, err
	}

	bytecodeHex := mapData["bytecode"].AsString()
	bytecodeHex = strings.TrimLeft(bytecodeHex, "0x")
	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		return nil, err
	}
	deployedBytecodeHex := mapData["deployedBytecode"].AsString()
	deployedBytecodeHex = strings.TrimLeft(deployedBytecodeHex, "0x")
	deployedBytecode, err := hex.DecodeString(deployedBytecodeHex)
	if err != nil {
		return nil, err
	}

	abi, err := abi.JSON(strings.NewReader(string(mapData["abi"])))
	if err != nil {
		return nil, err
	}
	return &ParsedABI{ABI: abi, Bytecode: bytecode, DeployedBytecode: deployedBytecode}, nil
}

func (p *ParsedABI) BytecodeHex() string {
	return hex.EncodeToString(p.Bytecode)
}
