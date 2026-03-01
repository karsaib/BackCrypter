package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type FileMeta struct {
	Size     int64 `json:"size"`
	ModUnix  int64 `json:"mod_unix"`
	SeenUnix int64 `json:"seen_unix"`
}

type Manifest struct {
	Version int                 `json:"version"`
	Files   map[string]FileMeta `json:"files"`
}

func manifestPath(targetDir string) string {
	return filepath.Join(targetDir, ".backcrypter", "manifest.json")
}

func Load(targetDir string) (*Manifest, error) {
	mp := manifestPath(targetDir)
	b, err := os.ReadFile(mp)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{Version: 1, Files: map[string]FileMeta{}}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Files == nil {
		m.Files = map[string]FileMeta{}
	}
	if m.Version == 0 {
		m.Version = 1
	}
	return &m, nil
}

func Save(targetDir string, m *Manifest) error {
	dir := filepath.Join(targetDir, ".backcrypter")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	mp := manifestPath(targetDir)
	tmp := mp + ".tmp"

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, mp)
}
