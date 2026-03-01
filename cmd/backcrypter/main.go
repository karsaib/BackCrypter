package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"backcrypter/internal/backup"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "backup":
		backupCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`backcrypter - incremental backup (file-level)

Usage:
  backcrypter backup --source <dir> --target <dir> [--exclude <csv>] [--dry-run]

Examples:
  backcrypter backup --source "/data" --target "/mnt/backup"
  backcrypter backup --source "/data" --target "/backup" --exclude ".git,node_modules,*.tmp"
`)
}

func backupCmd(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	source := fs.String("source", "", "Source folder")
	target := fs.String("target", "", "Target folder")
	exclude := fs.String("exclude", "", "Comma-separated exclude patterns")
	dryRun := fs.Bool("dry-run", false, "Print actions without copying")
	fs.Parse(args)

	if *source == "" || *target == "" {
		fmt.Println("ERROR: --source and --target are required")
		os.Exit(2)
	}

	var patterns []string
	if strings.TrimSpace(*exclude) != "" {
		for _, p := range strings.Split(*exclude, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				patterns = append(patterns, p)
			}
		}
	}

	sum, err := backup.Run(backup.Options{
		SourceDir: *source,
		TargetDir: *target,
		Exclude:   patterns,
		DryRun:    *dryRun,
	})
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}

	fmt.Printf("Done. scanned=%d copied=%d skipped=%d bytesCopied=%d\n",
		sum.Scanned, sum.Copied, sum.Skipped, sum.BytesCopied)
}
