// Package config, uygulama ayarlarını (settings.json) ve Keychain'deki
// oturum anahtarlarını yönetir.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

const appID = "ai-tempo"

// oldAppID, uygulamanın eski adıdır ("AI Kota Sorgulama"); ayarlar ve Keychain
// kayıtları bu addan yeni ada otomatik taşınır.
const oldAppID = "ai-kota-sorgulama"

// Account, tek bir servis hesabının (Claude, Cursor, ChatGPT, ...) bilgilerini tutar.
// SessionKey diske yazılmaz; macOS Keychain'de saklanır.
type Account struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"` // "claude" | "cursor" | "chatgpt"
	OrganizationID string `json:"organization_id"`
	Enabled        bool   `json:"enabled"`
	SessionKey     string `json:"-"`
}

// Settings, uygulamanın kalıcı ayarlarıdır
// (~/Library/Application Support/ai-tempo/settings.json).
type Settings struct {
	RefreshMinutes int       `json:"refresh_minutes"`
	Accounts       []Account `json:"accounts"`
}

const defaultRefreshMinutes = 5

// Current, uygulamanın o an yüklü ayarlarıdır (Load ile doldurulur).
var Current *Settings

func settingsPath() string { return settingsPathFor(appID) }

func settingsPathFor(id string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, id, "settings.json")
}

// migrateFromOldName, eski addaki ayar dosyasını ve Keychain anahtarlarını yeni
// ada taşır. Yeni ayar dosyası zaten varsa hiçbir şey yapmaz.
func migrateFromOldName() error {
	if _, err := os.Stat(settingsPath()); err == nil {
		return nil
	}
	oldPath := settingsPathFor(oldAppID)
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return nil // taşınacak eski ayar yok
	}
	var old Settings
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}
	for _, a := range old.Accounts {
		key, err := keychainGetFor(oldAppID, a.ID)
		if err != nil || key == "" {
			continue
		}
		if err := KeychainSet(a.ID, key); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath()), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath(), data, 0o600); err != nil {
		return err
	}
	// Yeni yere yazıldıktan sonra eski kayıtlar temizlenir.
	for _, a := range old.Accounts {
		_ = exec.Command("security", "delete-generic-password", "-s", oldAppID, "-a", a.ID).Run()
	}
	return os.RemoveAll(filepath.Dir(oldPath))
}

// Load, ayarları okur ve anahtarları Keychain'den doldurur. Ayar dosyası
// yoksa eski config.json bulunursa ondan içe aktarır (anahtarlar Keychain'e taşınır).
func Load() (*Settings, error) {
	if err := migrateFromOldName(); err != nil {
		return nil, err
	}
	s := &Settings{RefreshMinutes: defaultRefreshMinutes}
	data, err := os.ReadFile(settingsPath())
	switch {
	case err == nil:
		if err := json.Unmarshal(data, s); err != nil {
			return nil, err
		}
		for i := range s.Accounts {
			s.Accounts[i].SessionKey, _ = keychainGet(s.Accounts[i].ID)
		}
	case errors.Is(err, os.ErrNotExist):
		if legacy, ok := loadLegacyConfig(); ok {
			s.Accounts = legacy
			if err := s.Save(); err != nil {
				return nil, err
			}
			for _, a := range s.Accounts {
				if err := KeychainSet(a.ID, a.SessionKey); err != nil {
					return nil, err
				}
			}
		}
	default:
		return nil, err
	}
	if s.RefreshMinutes <= 0 {
		s.RefreshMinutes = defaultRefreshMinutes
	}
	return s, nil
}

// save, ayarları (anahtarlar hariç) diske yazar.
func (s *Settings) Save() error {
	p := settingsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func (s *Settings) EnabledAccounts() []Account {
	var out []Account
	for _, a := range s.Accounts {
		if a.Enabled {
			out = append(out, a)
		}
	}
	return out
}

func EnabledAccounts() []Account { return Current.EnabledAccounts() }

func NewAccountID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Eski config.json içe aktarımı ---

// LegacyConfigPath, içe aktarılan eski config.json'un yolu (kullanıcıya silmesini hatırlatmak için).
var LegacyConfigPath string

func loadLegacyConfig() ([]Account, bool) {
	var legacy struct {
		Accounts []struct {
			Name           string `json:"name"`
			Provider       string `json:"provider"`
			SessionKey     string `json:"session_key"`
			OrganizationID string `json:"organization_id"`
			Enabled        bool   `json:"enabled"`
		} `json:"accounts"`
	}
	for _, path := range legacyConfigCandidates() {
		data, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(data, &legacy) != nil {
			continue
		}
		LegacyConfigPath, _ = filepath.Abs(path)
		accounts := make([]Account, 0, len(legacy.Accounts))
		for _, a := range legacy.Accounts {
			accounts = append(accounts, Account{
				ID: NewAccountID(), Name: a.Name, Provider: a.Provider,
				OrganizationID: a.OrganizationID, Enabled: a.Enabled, SessionKey: a.SessionKey,
			})
		}
		return accounts, true
	}
	return nil, false
}

// legacyConfigCandidates, eski config.json'un aranacağı yolları döndürür. .app
// olarak açıldığında çalışma dizini "/" olduğundan göreli yol tek başına yetmez.
func legacyConfigCandidates() []string {
	const filename = "config.json"
	paths := []string{filename}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		paths = append(paths,
			filepath.Join(dir, filename),                   // binary'nin yanı
			filepath.Join(dir, "..", "..", "..", filename), // .app'in bulunduğu klasör
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", oldAppID, filename))
	}
	return paths
}
