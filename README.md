# BackCrypter

BackCrypter is a lightweight, cross-platform incremental backup tool written in Go.

It performs fast file-level incremental backups by copying only new or modified files.  
Designed to be simple, reliable, and extensible — future versions will include encryption support.

---

## ✨ Features

- Cross-platform (Windows & Linux)
- Incremental file-level backup
- Manifest-based state tracking
- Exclude patterns support (glob & directory match)
- Atomic file writes (safe copy with temp + rename)
- Simple CLI interface
- No external dependencies

---

## 🚀 Installation

Clone the repository:

```bash
git clone <your-repo-url>
cd BackCrypter
