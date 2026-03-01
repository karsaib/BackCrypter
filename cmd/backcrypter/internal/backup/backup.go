package backup

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backcrypter/internal/manifest"
)

type Options struct {
	SourceDir string
	TargetDir string
	Exclude   []string // glob patterns or dir names
	DryRun    bool
}

type Summary struct {
	Scanned     int
	Copied      int
	Skipped     int
	BytesCopied int64
}

func Run(opts Options) (Summary, error) {
	var sum Summary

	src := filepath.Clean(opts.SourceDir)
	dst := filepath.Clean(opts.TargetDir)

	srcInfo, err := os.Stat(src)
	if err != nil {
		return sum, fmt.Errorf("source stat failed: %w", err)
	}
	if !srcInfo.IsDir() {
		return sum, errors.New("source is not a directory")
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return sum, fmt.Errorf("create target dir failed: %w", err)
	}

	// Load manifest from target
	m, err := manifest.Load(dst)
	if err != nil {
		return sum, fmt.Errorf("load manifest failed: %w", err)
	}

	now := time.Now().UTC()

	walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Continue scanning even if some file fails; return error to stop if you prefer strict mode
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel) // stable key format
		if rel == "." {
			return nil
		}

		// Exclude handling (directories and files)
		if shouldExclude(rel, d.IsDir(), opts.Exclude) {
			if d.IsDir() {
				return fs.SkipDir
			}
			sum.Skipped++
			return nil
		}

		// Only files
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		sum.Scanned++

		dstPath := filepath.Join(dst, filepath.FromSlash(rel))
		needCopy := true

		// Decide based on manifest first (fast)
		if prev, ok := m.Files[rel]; ok {
			// If size and modtime match, skip
			if prev.Size == info.Size() && prev.ModUnix == info.ModTime().UTC().Unix() {
				needCopy = false
			}
		} else {
			// If not in manifest, still can skip if target already exists and matches (optional)
			if st, err := os.Stat(dstPath); err == nil && !st.IsDir() {
				if st.Size() == info.Size() {
					// If target modtime equals source modtime -> likely unchanged
					if st.ModTime().UTC().Unix() == info.ModTime().UTC().Unix() {
						needCopy = false
					}
				}
			}
		}

		if !needCopy {
			sum.Skipped++
			// Refresh manifest "seen" timestamp (optional)
			m.Files[rel] = manifest.FileMeta{Size: info.Size(), ModUnix: info.ModTime().UTC().Unix(), SeenUnix: now.Unix()}
			return nil
		}

		if opts.DryRun {
			fmt.Println("[DRY] COPY", path, "->", dstPath)
			sum.Copied++
			m.Files[rel] = manifest.FileMeta{Size: info.Size(), ModUnix: info.ModTime().UTC().Unix(), SeenUnix: now.Unix()}
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}

		n, err := copyFile(path, dstPath, info.Mode())
		if err != nil {
			return err
		}

		// Preserve modtime
		_ = os.Chtimes(dstPath, time.Now(), info.ModTime())

		sum.Copied++
		sum.BytesCopied += n

		m.Files[rel] = manifest.FileMeta{Size: info.Size(), ModUnix: info.ModTime().UTC().Unix(), SeenUnix: now.Unix()}
		return nil
	})

	if walkErr != nil {
		return sum, walkErr
	}

	// Save manifest
	if err := manifest.Save(dst, m); err != nil {
		return sum, fmt.Errorf("save manifest failed: %w", err)
	}

	return sum, nil
}

func shouldExclude(rel string, isDir bool, patterns []string) bool {
	// rel is forward-slash style
	parts := strings.Split(rel, "/")
	base := parts[len(parts)-1]

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Directory name exclude: "node_modules" -> exclude any folder with that name
		if isDir && p == base {
			return true
		}
		// File name exclude exact: ".DS_Store"
		if !isDir && p == base {
			return true
		}
		// Glob match against rel and base
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
	}
	return false
}

func copyFile(srcPath, dstPath string, mode fs.FileMode) (int64, error) {
	in, err := os.Open(srcPath)
	if err != nil {
		return 0, err
	}
	defer in.Close()

	// Write to temp then atomic rename
	tmp := dstPath + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return 0, err
	}

	n, copyErr := io.Copy(out, in)
	closeErr := out.Close()

	if copyErr != nil {
		_ = os.Remove(tmp)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return 0, closeErr
	}

	if err := os.Rename(tmp, dstPath); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}
