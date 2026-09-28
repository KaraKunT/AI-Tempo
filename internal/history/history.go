// Package history, her hesabın sorgu geçmişini (başarılı/hatalı, süre, mesaj)
// ve grafik için kota ölçümlerini SQLite'ta tutar. Saklama süresi ayarlardan
// belirlenir (SetRetention).
package history

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// retention, kayıtların ve ölçümlerin saklanma süresidir.
var retention = 35 * 24 * time.Hour

// SetRetention, saklama süresini gün olarak ayarlar ve eski kayıtları siler.
func SetRetention(days int) {
	mu.Lock()
	retention = time.Duration(days) * 24 * time.Hour
	mu.Unlock()
	prune()
}

// Sample, bir kota göstergesinin belirli bir andaki kullanım yüzdesidir.
type Sample struct {
	Metric  string // gösterge ID'si (yalnızca List'in döndürdüğü kayıtlarda dolu)
	At      time.Time
	Percent float64
}

// Entry, tek bir sorgu kaydıdır.
type Entry struct {
	At       time.Time
	Success  bool
	Trigger  string // "Otomatik", "Elle", "Sıfırlama"
	Message  string // hata mesajı veya kota özeti
	Duration time.Duration
	Metrics  []Sample // başarılı sorgunun gösterge yüzdeleri (aynı andaki ölçümler)
}

var (
	mu sync.Mutex
	db *sql.DB
)

// Open, veritabanını dir/history.db'de açar (yoksa oluşturur) ve eski kayıtları siler.
func Open(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	d, err := sql.Open("sqlite", filepath.Join(dir, "history.db"))
	if err != nil {
		return err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS logs (
		id          INTEGER PRIMARY KEY,
		account_id  TEXT    NOT NULL,
		at          INTEGER NOT NULL,
		success     INTEGER NOT NULL,
		trigger     TEXT    NOT NULL,
		message     TEXT    NOT NULL,
		duration_ms INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS logs_account_at ON logs(account_id, at);
	CREATE TABLE IF NOT EXISTS samples (
		account_id TEXT    NOT NULL,
		label      TEXT    NOT NULL,
		at         INTEGER NOT NULL,
		percent    REAL    NOT NULL
	);
	CREATE INDEX IF NOT EXISTS samples_account_label_at ON samples(account_id, label, at);`); err != nil {
		d.Close()
		return err
	}
	// Eski sürümler ölçümleri Türkçe gösterge adıyla saklıyordu; dilden bağımsız ID'ye çevir.
	d.Exec(`UPDATE samples SET label = CASE label
		WHEN 'Mevcut Oturum'   THEN 'session'
		WHEN '5 Saatlik Limit' THEN 'session'
		WHEN 'Haftalık Limit'  THEN 'weekly'
		WHEN 'Kullanım Limiti' THEN 'primary'
		WHEN 'İkincil Limit'   THEN 'secondary'
		WHEN 'Toplam Kullanım' THEN 'total'
		WHEN 'Otomatik Model'  THEN 'auto'
		WHEN 'API Kullanımı'   THEN 'api'
		ELSE label END`)
	mu.Lock()
	db = d
	mu.Unlock()
	prune()
	startPruner()
	return nil
}

// Add, bir hesaba sorgu kaydı ekler. Veritabanı açılamadıysa sessizce yok sayılır.
func Add(accountID string, e Entry) {
	mu.Lock()
	defer mu.Unlock()
	if db == nil {
		return
	}
	db.Exec(`INSERT INTO logs(account_id, at, success, trigger, message, duration_ms) VALUES (?,?,?,?,?,?)`,
		accountID, e.At.UnixMilli(), e.Success, e.Trigger, e.Message, e.Duration.Milliseconds())
}

// List, hesabın [from, to) aralığındaki kayıtlarını en yeni başta olacak şekilde döndürür.
func List(accountID string, from, to time.Time) []Entry {
	mu.Lock()
	defer mu.Unlock()
	if db == nil {
		return nil
	}
	rows, err := db.Query(`SELECT at, success, trigger, message, duration_ms FROM logs
		WHERE account_id = ? AND at >= ? AND at < ? ORDER BY at DESC`,
		accountID, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var at, ms int64
		var e Entry
		if rows.Scan(&at, &e.Success, &e.Trigger, &e.Message, &ms) == nil {
			e.At = time.UnixMilli(at)
			e.Duration = time.Duration(ms) * time.Millisecond
			out = append(out, e)
		}
	}
	rows.Close()

	// Her kayda, aynı anda kaydedilmiş gösterge ölçümlerini ekle.
	srows, err := db.Query(`SELECT label, at, percent FROM samples
		WHERE account_id = ? AND at >= ? AND at < ? ORDER BY rowid`,
		accountID, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return out
	}
	defer srows.Close()
	byAt := map[int64][]Sample{}
	for srows.Next() {
		var s Sample
		var at int64
		if srows.Scan(&s.Metric, &at, &s.Percent) == nil {
			s.At = time.UnixMilli(at)
			byAt[at] = append(byAt[at], s)
		}
	}
	for i := range out {
		out[i].Metrics = byAt[out[i].At.UnixMilli()]
	}
	return out
}

// AddSample, bir kota göstergesinin (metricID) ölçümünü kaydeder.
func AddSample(accountID, metricID string, at time.Time, percent float64) {
	mu.Lock()
	defer mu.Unlock()
	if db == nil {
		return
	}
	db.Exec(`INSERT INTO samples(account_id, label, at, percent) VALUES (?,?,?,?)`,
		accountID, metricID, at.UnixMilli(), percent)
}

// Samples, göstergenin [from, to) aralığındaki ölçümlerini eskiden yeniye döndürür.
func Samples(accountID, metricID string, from, to time.Time) []Sample {
	mu.Lock()
	defer mu.Unlock()
	if db == nil {
		return nil
	}
	rows, err := db.Query(`SELECT at, percent FROM samples
		WHERE account_id = ? AND label = ? AND at >= ? AND at < ? ORDER BY at`,
		accountID, metricID, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Sample
	for rows.Next() {
		var at int64
		var p float64
		if rows.Scan(&at, &p) == nil {
			out = append(out, Sample{At: time.UnixMilli(at), Percent: p})
		}
	}
	return out
}

// Delete, silinen bir hesabın tüm kayıtlarını kaldırır.
func Delete(accountID string) {
	mu.Lock()
	defer mu.Unlock()
	if db != nil {
		db.Exec(`DELETE FROM logs WHERE account_id = ?`, accountID)
		db.Exec(`DELETE FROM samples WHERE account_id = ?`, accountID)
	}
}

// prune, saklama süresinden eski kayıtları siler.
func prune() {
	mu.Lock()
	defer mu.Unlock()
	if db == nil {
		return
	}
	cutoff := time.Now().Add(-retention).UnixMilli()
	db.Exec(`DELETE FROM logs WHERE at < ?`, cutoff)
	db.Exec(`DELETE FROM samples WHERE at < ?`, cutoff)
}

// startPruner, uygulama uzun süre açık kaldığında da eski kayıtları saatte bir temizler.
func startPruner() {
	go func() {
		for range time.Tick(time.Hour) {
			prune()
		}
	}()
}
