// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/codec"
)

func isJSONOutputRequested(cmd *cobra.Command) (bool, error) {
	output, err := getConfigValue(cmd, "output")
	if err != nil {
		return false, fmt.Errorf("failed to get output format: %w", err)
	}
	return strings.ToLower(output) == "json", nil
}

func printValue(cmd *cobra.Command, v fmt.Stringer) error {
	isJSON, err := isJSONOutputRequested(cmd)
	if err != nil {
		return err
	}

	if isJSON {
		jsonBytes, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	} else {
		fmt.Println(v.String())
		return nil
	}
}

func getConfigValue(cmd *cobra.Command, name string) (string, error) {
	// Check if the value is among flags
	if value, err := cmd.Flags().GetString(name); err == nil && value != "" {
		return value, nil
	}

	// If not in flags, check the config file
	config, err := readConfig()
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}

	if value, ok := config[name]; ok {
		return value, nil
	}

	return "", fmt.Errorf("value for %s not found", name)
}

func updateConfig(name, value string) error {
	config, err := readConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	config[name] = value
	return writeConfig(config)
}

func readConfig() (map[string]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".hypersdk-cli", "config.cfg")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return config, nil
}

func writeConfig(config map[string]string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".hypersdk-cli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.cfg")
	var buf strings.Builder
	for key, value := range config {
		buf.WriteString(fmt.Sprintf("%s = %s\n", key, value))
	}

	if err := os.WriteFile(configPath, []byte(buf.String()), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func decodeFileOrHex(whatever string) ([]byte, error) {
	if decoded, err := codec.LoadHex(whatever, -1); err == nil {
		return decoded, nil
	}

	if fileContents, err := os.ReadFile(whatever); err == nil {
		return fileContents, nil
	}

	return nil, errors.New("unable to decode input as hex, or read as file path")
}
