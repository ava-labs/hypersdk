package v2

import (
	"fmt"
	"os"
)

func NewLogModule() *ImportModule {
	return &ImportModule{name: "log",
		funcs: map[string]Function{
			"debug": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				return log("DEBUG", input)
			},
			"info": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				return log("INFO", input)
			},
			"warn": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				return log("WARN", input)
			},
			"error": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				return log("ERROR", input)
			},
		},
	}
}

func log(level string, logBytes []byte) ([]byte, error) {
	_, err := fmt.Fprintf(os.Stderr, "%s:%s\n", level, logBytes)
	return nil, err
}
