//go:build windows

package config

import (
	"errors"

	"github.com/danieljoos/wincred"
)

// Windows'ta anahtarlar Kimlik Bilgisi Yöneticisi'nde (Credential Manager)
// "ai-tempo/<hesap-id>" adıyla saklanır.

func credName(service, id string) string { return service + "/" + id }

func KeychainSet(id, secret string) error {
	c := wincred.NewGenericCredential(credName(appID, id))
	c.CredentialBlob = []byte(secret)
	c.Persist = wincred.PersistLocalMachine
	return c.Write()
}

func keychainGet(id string) (string, error) { return keychainGetFor(appID, id) }

func keychainGetFor(service, id string) (string, error) {
	c, err := wincred.GetGenericCredential(credName(service, id))
	if err != nil {
		return "", err
	}
	if c == nil {
		return "", errors.New("kimlik bilgisi bulunamadı")
	}
	return string(c.CredentialBlob), nil
}

func KeychainDelete(id string) { keychainDeleteFor(appID, id) }

func keychainDeleteFor(service, id string) {
	if c, err := wincred.GetGenericCredential(credName(service, id)); err == nil && c != nil {
		_ = c.Delete()
	}
}
