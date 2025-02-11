package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourpwnguy/refine/pkg/common"
)

// isDir checks if the given path is a directory.
func IsDir(dir string) bool {
	info, _ := os.Stat(dir)
	return info.IsDir()
}

// HandleVersionAndHelp processes the command-line arguments to check for version and help flags.
func HandleVersionAndHelp() {
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" {
			common.CheckVersion()
			os.Exit(0)
		} else if arg == "-h" || arg == "--help" {
			common.PrintUsage()
			os.Exit(0)
		}
	}
}

// IsInputFromStdin checks if the input is being provided from the standard input (stdin).
func IsInputFromStdin() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// shouldSkip checks if a file should be skipped based on the exceptions list.
func ShouldSkip(fileName string, exceptions []string) bool {
	for _, exception := range exceptions {
		if fileName == filepath.Base(exception) {
			return true
		}
	}
	return false
}

// A way to ensure that file exists to open else create it
func OpenOrCreateFile(fn string, toBeCreated bool) (*os.File, error) {
	_, err := os.Stat(fn)
	if os.IsNotExist(err) {
		if toBeCreated {
			return os.Create(fn)
		}
		return nil, err
	}
	return os.OpenFile(fn, os.O_RDWR|os.O_APPEND, 0644)
}

// Before using it instantiate a map using make() and then pass it
func ReadLinesFromFileAndStdin(fn *os.File, targetMap map[string]struct{}, stdinMap map[string]struct{}) (int, error) {

	// Counter
	totalLinesCount := 0

	data, err := os.ReadFile(fn.Name())
	if err != nil {
		return 0, fmt.Errorf("%s Error reading file: %w", common.Errfix, err)
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		targetMap[trimmedLine] = struct{}{}
		totalLinesCount += 1
	}

	for line := range stdinMap {
		targetMap[line] = struct{}{}
	}

	return totalLinesCount, nil
}

func OpenAndTruncate(f string) (*os.File, error) {
	return os.OpenFile(f, os.O_RDWR|os.O_TRUNC, 0644)
}
