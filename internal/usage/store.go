// Package usage, tüm hesapların kota sorgularını tek bir yerden, aralıklı ve
// sırayla yapan zamanlayıcıyı içerir.
package usage

import (
	"context"
	"sync"
	"time"

	"ai-tempo/internal/config"
	"ai-tempo/internal/provider"
)

// Sorgu politikası: servisleri yormamak (ve engellenmemek) için tüm sorgular
// tek bir yerden, sırayla ve aralıklı yapılır.
const (
	pollTick       = time.Minute      // zamanı gelen hesaplar bu sıklıkla kontrol edilir
	manualMinGap   = 30 * time.Second // elle yenilemede aynı hesap için en kısa aralık
	requestSpacing = 1500 * time.Millisecond
	// MaxAutoRetries, üst üste bu kadar hatalı sorgudan sonra hesap otomatik
	// olarak sorgulanmaz; elle yenileme veya sayacı sıfırlama gerekir.
	MaxAutoRetries = 5
)

// Store, tüm hesapların son kota sonuçlarını tutar; pencere ve menü çubuğu
// aynı sonuçları paylaşır, böylece her hesap dönem başına bir kez sorgulanır.
type Store struct {
	mu        sync.Mutex
	results   map[string]*provider.RateLimitInfo
	fetched   map[string]time.Time
	failures  map[string]int
	loading   map[string]bool
	updated   time.Time
	listeners []func()
}

// Default, uygulamanın tek paylaşılan sorgu deposudur.
var Default = &Store{
	results:  map[string]*provider.RateLimitInfo{},
	fetched:  map[string]time.Time{},
	failures: map[string]int{},
	loading:  map[string]bool{},
}

// OnChange, sonuçlar değiştiğinde çağrılacak bir dinleyici ekler. Dinleyici
// arka plan goroutine'inden çağrılabilir; UI güncellemesi yapacaksa fyne.Do kullanmalıdır.
func (u *Store) OnChange(f func()) { u.listeners = append(u.listeners, f) }

func (u *Store) Notify() {
	for _, f := range u.listeners {
		f()
	}
}

// Get, bir hesabın son sonucunu ve şu an sorgulanıp sorgulanmadığını döndürür.
func (u *Store) Get(id string) (*provider.RateLimitInfo, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.results[id], u.loading[id]
}

func (u *Store) Status() (loading bool, updated time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, l := range u.loading {
		if l {
			return true, u.updated
		}
	}
	return false, u.updated
}

// Failures, hesabın üst üste başarısız sorgu sayısını döndürür.
func (u *Store) Failures(id string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.failures[id]
}

// ResetFailures, hesabın hata sayacını sıfırlar ve hesabı hemen yeniden sorgular.
func (u *Store) ResetFailures(account config.Account) {
	u.mu.Lock()
	u.failures[account.ID] = 0
	delete(u.fetched, account.ID)
	u.mu.Unlock()
	u.Refresh([]config.Account{account}, false)
}

// Forget, silinen/değiştirilen bir hesabın önbelleğini temizler.
func (u *Store) Forget(id string) {
	u.mu.Lock()
	delete(u.results, id)
	delete(u.fetched, id)
	delete(u.failures, id)
	u.mu.Unlock()
}

// Start, arka planda zamanı gelen hesapları sorgulayan döngüyü başlatır.
func (u *Store) Start() {
	u.Refresh(config.EnabledAccounts(), false)
	go func() {
		for range time.Tick(pollTick) {
			u.Refresh(config.EnabledAccounts(), false)
		}
	}()
}

// Refresh, verilen hesapları sırayla sorgular. force=false ise yalnızca
// zamanı gelenler (yenileme aralığı dolmuş ve hata sayacı MaxAutoRetries'a
// ulaşmamış olanlar) sorgulanır;
// force=true ise (elle yenileme) son 30 sn'de sorgulanmamış olanlar sorgulanır.
// Sorguya alınan hesap sayısını döndürür.
func (u *Store) Refresh(accounts []config.Account, force bool) int {
	now := time.Now()
	interval := time.Duration(config.Current.RefreshMinutes) * time.Minute

	var due []config.Account
	u.mu.Lock()
	for _, a := range accounts {
		if u.loading[a.ID] {
			continue
		}
		last, seen := u.fetched[a.ID]
		wait := interval
		if force {
			wait = manualMinGap
			if r := u.results[a.ID]; r != nil && !r.Success {
				wait = 0 // hatalı hesap elle hemen yeniden denenebilir
			}
		} else if u.failures[a.ID] >= MaxAutoRetries {
			continue
		}
		if seen && now.Sub(last) < wait {
			continue
		}
		u.loading[a.ID] = true
		due = append(due, a)
	}
	u.mu.Unlock()
	if len(due) == 0 {
		return 0
	}
	u.Notify()

	go func() {
		for i, a := range due {
			if i > 0 {
				time.Sleep(requestSpacing)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			info := provider.Get(a.Provider).Query(ctx, a)
			cancel()

			u.mu.Lock()
			u.results[a.ID] = &info
			u.fetched[a.ID] = time.Now()
			u.loading[a.ID] = false
			if info.Success {
				u.failures[a.ID] = 0
			} else {
				u.failures[a.ID]++
			}
			u.updated = time.Now()
			u.mu.Unlock()
			u.Notify()
		}
	}()
	return len(due)
}
