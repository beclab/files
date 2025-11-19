package implementations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MbLinkShortcutHandler struct{}

func (h *MbLinkShortcutHandler) Extension() string {
	return ".mblink"
}

func (h *MbLinkShortcutHandler) Resolve(shortcutPath string) (string, error) {
	if shortcutPath == "" {
		return "", fmt.Errorf("shortcutPath cannot be empty")
	}

	if filepath.Ext(shortcutPath) != ".mblink" {
		return "", nil
	}

	target, err := os.ReadFile(shortcutPath)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(string(target), string(os.PathSeparator)), nil
}

func (h *MbLinkShortcutHandler) Create(shortcutPath, targetPath string) error {
	if shortcutPath == "" {
		return fmt.Errorf("shortcutPath cannot be empty")
	}
	if targetPath == "" {
		return fmt.Errorf("targetPath cannot be empty")
	}

	return os.WriteFile(shortcutPath, []byte(targetPath), 0644)
}
