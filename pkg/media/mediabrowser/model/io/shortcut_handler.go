package io

type ShortcutHandler interface {
	Extension() string
	Resolve(shortcutPath string) (string, error)
	Create(shortcutPath, targetPath string) error
}
