package v2

import (
	"fmt"
	"os"
)

func NewLogModule() *ImportModule {
	return &ImportModule{name: "log",
		funcs: map[string]HostFunction{
			"debug": FunctionNoOutput(func(_ *CallInfo, input []byte) error { return log("DEBUG", input) }),
			"info": FunctionNoOutput(func(_ *CallInfo, input []byte) error {
				return log("INFO", input)
			}),
			"warn": FunctionNoOutput(func(_ *CallInfo, input []byte) error {
				return log("WARN", input)
			}),
			"error": FunctionNoOutput(func(_ *CallInfo, input []byte) error {
				return log("ERROR", input)
			}),
		},
	}
}

func log(level string, logBytes []byte) error {
	_, err := fmt.Fprintf(os.Stderr, "%s:%s\n", level, logBytes)
	return err
}
