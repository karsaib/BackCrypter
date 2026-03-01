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
	Exclude   []string
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

	m, err := manifest.Load(dst)
	if err != nil {
		return sum, fmt.Errorf("load manifest failed: %w", err)
	}

	now := time.Now().UTC()

	walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}

		if shouldExclude(rel, d.IsDir(), opts.Exclude) {
			if d.IsDir() {
				return fs.SkipDir
			}
			sum.Skipped++
			return nil
		}

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
		if prev, ok := m.Files[rel]; ok {
			if prev.Size == info.Size() && prev.ModUnix == info.ModTime().UTC().Unix() {
				needCopy = false
			}
		}

		if !needCopy {
			sum.Skipped++
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

		_ = os.Chtimes(dstPath, time.Now(), info.ModTime())

		sum.Copied++
		sum.BytesCopied += n
		m.Files[rel] = manifest.FileMeta{Size: info.Size(), ModUnix: info.ModTime().UTC().Unix(), SeenUnix: now.Unix()}
		return nil
	})

	if walkErr != nil {
		return sum, walkErr
	}

	if err := manifest.Save(dst, m); err != nil {
		return sum, fmt.Errorf("save manifest failed: %w", err)
	}

	return sum, nil
}

func shouldExclude(rel string, isDir bool, patterns []string) bool {
	parts := strings.Split(rel, "/")
	base := parts[len(parts)-1]

	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if isDir && p == base {
			return true
		}
		if !isDir && p == base {
			return true
		}
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
