package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/marcusz/monitoring-assistant/pkg/kb"
)

func main() {
	var kbPath string
	var outPath string
	flag.StringVar(&kbPath, "kb-path", "KB", "Path to KB folder")
	flag.StringVar(&outPath, "out", "", "Output index path (default: KB/.kb_index.json)")
	flag.Parse()

	if kbPath == "" {
		log.Fatal("kb-path is required")
	}
	if outPath == "" {
		outPath = kb.DefaultIndexPath(kbPath)
	}

	idx, err := kb.BuildIndex(kbPath)
	if err != nil {
		log.Fatalf("build index: %v", err)
	}

	if err := kb.SaveIndex(outPath, idx); err != nil {
		log.Fatalf("save index: %v", err)
	}

	fmt.Printf("KB index written to %s (sections=%d)\n", outPath, len(idx.Sections))

	// Tighten permissions if possible.
	if err := os.Chmod(outPath, 0o600); err != nil {
		// Best effort; no fatal.
	}
}
