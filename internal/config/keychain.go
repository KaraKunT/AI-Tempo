package config

import (
	"os/exec"
	"strings"
)

// Keychain erişimi macOS `security` aracı üzerinden yapılır (cgo gerektirmez).

func KeychainSet(id, secret string) error {
	return exec.Command("security", "add-generic-password", "-U", "-s", appID, "-a", id, "-w", secret).Run()
}

func keychainGet(id string) (string, error) { return keychainGetFor(appID, id) }

func keychainGetFor(service, id string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", id, "-w").Output()
	return strings.TrimSpace(string(out)), err
}

func KeychainDelete(id string) {
	_ = exec.Command("security", "delete-generic-password", "-s", appID, "-a", id).Run()
}
