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
	"github.com/spf13/viper"

	"github.com/ava-labs/hypersdk/codec"
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting home directory:", err)
		os.Exit(1)
	}

	configDir := filepath.Join(homeDir, ".hypersdk-cli")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "Error creating config directory:", err)
		os.Exit(1)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if _, err := os.Create(configFile); err != nil {
			fmt.Fprintln(os.Stderr, "Error creating config file:", err)
			os.Exit(1)
		}
	}

	// Set config name and paths
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintln(os.Stderr, "Error reading config:", err)
			os.Exit(1)
		}
		// Config file not found; will be created when needed
	}
}

func isJSONOutputRequested(cmd *cobra.Command) (bool, error) {
	output, err := getConfigValue(cmd, "output", false)
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

func getConfigValue(cmd *cobra.Command, key string, required bool) (string, error) {
	// Check flags first
	if value, err := cmd.Flags().GetString(key); err == nil && value != "" {
		return value, nil
	}

	// Then check viper
	if value := viper.GetString(key); value != "" {
		return value, nil
	}

	if required {
		return "", fmt.Errorf("required value for %s not found", key)
	}

	return "", nil
}

func setConfigValue(key, value string) error {
	viper.Set(key, value)
	return viper.WriteConfig()
}

func decodeFileOrHex(fileNameOrHex string) ([]byte, error) {
	if decoded, err := codec.LoadHex(fileNameOrHex, -1); err == nil {
		return decoded, nil
	}

	if fileContents, err := os.ReadFile(fileNameOrHex); err == nil {
		return fileContents, nil
	}

	return nil, errors.New("unable to decode input as hex, or read as file path")
}
