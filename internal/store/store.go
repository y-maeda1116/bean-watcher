// Package store は config/data の JSON ファイル読み書きを行う。
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"bean-watcher/internal/model"
)

// LoadConfig は config.json を読み込む。
func LoadConfig(path string) (model.Config, error) {
	var c model.Config
	if err := readJSON(path, &c); err != nil {
		return model.Config{}, err
	}
	return c, nil
}

// SaveConfig は config.json を書き込む（テスト・初期生成用）。
func SaveConfig(path string, c model.Config) error {
	return writeJSON(path, c)
}

// LoadData は data.json を読み込む。
func LoadData(path string) (model.Data, error) {
	var d model.Data
	if err := readJSON(path, &d); err != nil {
		return model.Data{}, err
	}
	return d, nil
}

// SaveData は data.json を書き込む（インデント付き）。
func SaveData(path string, d model.Data) error {
	return writeJSON(path, d)
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}
