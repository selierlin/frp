// Copyright 2025 The frp Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"encoding/json"
	"os"

	toml "github.com/pelletier/go-toml/v2"
	"sigs.k8s.io/yaml"

	v1 "github.com/fatedier/frp/pkg/config/v1"
)

// SaveServerConfigToFile saves the ServerConfig to a file in the specified format.
func SaveServerConfigToFile(path string, cfg *v1.ServerConfig) error {
	format := DetectFormatFromPath(path)
	content, err := encodeServerConfig(cfg, format)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}

// SaveServerConfigPartial updates specific fields in the config file.
// Strategy: read original file → parse → update specified fields → encode in original format → write back.
func SaveServerConfigPartial(path string, updates func(*v1.ServerConfig)) error {
	format := DetectFormatFromPath(path)
	if format == "" {
		format = "toml" // default to toml if format cannot be detected
	}

	// 1. Read original file content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// 2. Parse into ServerConfig
	cfg := &v1.ServerConfig{}
	if err := LoadConfigure(content, cfg, false, format); err != nil {
		return err
	}

	// 3. Apply updates
	updates(cfg)

	// 4. Encode in original format
	newContent, err := encodeServerConfig(cfg, format)
	if err != nil {
		return err
	}

	// 5. Write back to file
	return os.WriteFile(path, newContent, 0o600)
}

// encodeServerConfig encodes ServerConfig to bytes in the specified format.
func encodeServerConfig(cfg *v1.ServerConfig, format string) ([]byte, error) {
	switch format {
	case "toml":
		return toml.Marshal(cfg)
	case "yaml", "yml":
		return yaml.Marshal(cfg)
	case "json":
		return json.MarshalIndent(cfg, "", "  ")
	default:
		// default to toml
		return toml.Marshal(cfg)
	}
}