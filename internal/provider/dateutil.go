package provider

import (
	"fmt"
	"strconv"
	"time"
)

var turkishMonths = []string{"Oca", "Şub", "Mar", "Nis", "May", "Haz", "Tem", "Ağu", "Eyl", "Eki", "Kas", "Ara"}
var turkishWeekdays = []string{"Pazar", "Pazartesi", "Salı", "Çarşamba", "Perşembe", "Cuma", "Cumartesi"}

// formatResetTime, RFC3339 formatındaki bir reset zamanını "X gün Y saat | 23 Eyl Salı 14:59"
// biçiminde, Türkiye saatine (Europe/Istanbul) çevrilmiş olarak döndürür.
func formatResetTime(resetStr string) string {
	resetsTime, err := time.Parse(time.RFC3339, resetStr)
	if err != nil {
		return ""
	}
	return formatResetTimeAt(resetsTime)
}

// formatResetTimeUnixMilli, unix milisaniye cinsinden bir zaman damgasını
// formatResetTime ile aynı biçimde döndürür (Cursor gibi ms epoch dönen servisler için).
func formatResetTimeUnixMilli(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return formatResetTimeAt(time.UnixMilli(ms))
}

// formatResetTimeUnixSeconds, unix saniye cinsinden bir zaman damgasını
// formatResetTime ile aynı biçimde döndürür (ChatGPT gibi saniye epoch dönen servisler için).
func formatResetTimeUnixSeconds(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return formatResetTimeAt(time.Unix(sec, 0))
}

// formatResetTimeAt, verilen zamanı şimdiye göre "X gün Y saat | tarih" biçiminde yazar.
func formatResetTimeAt(resetsTime time.Time) string {
	now := time.Now().UTC()
	duration := resetsTime.UTC().Sub(now)

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	mins := int(duration.Minutes()) % 60

	loc, _ := time.LoadLocation("Europe/Istanbul")
	turkishTime := resetsTime.In(loc)

	switch {
	case days > 0:
		return fmt.Sprintf("%d gün %d saat | %s", days, hours, formatTurkishDate(turkishTime))
	case hours > 0:
		return fmt.Sprintf("%d saat %d dakika | %s", hours, mins, formatTurkishDate(turkishTime))
	default:
		return fmt.Sprintf("%d dakika | %s", mins, formatTurkishDate(turkishTime))
	}
}

// formatTurkishDate, bir zamanı "23 Eyl Salı 14:59" formatında Türkçe olarak yazar.
func formatTurkishDate(t time.Time) string {
	return fmt.Sprintf("%d %s %s %02d:%02d",
		t.Day(), turkishMonths[t.Month()-1], turkishWeekdays[t.Weekday()], t.Hour(), t.Minute())
}

// timeProgress, bir pencerenin reset zamanı ve toplam uzunluğuna göre şimdiye
// kadar geçen sürenin oranını (0-1) döndürür. Reset zamanı bilinmiyor veya
// pencere uzunluğu geçersizse -1 döner (UI'da göstermemek için).
func timeProgress(resetAt time.Time, windowSeconds float64) float64 {
	if resetAt.IsZero() || windowSeconds <= 0 {
		return -1
	}
	remaining := resetAt.UTC().Sub(time.Now().UTC()).Seconds()
	if remaining < 0 {
		remaining = 0
	}
	if remaining > windowSeconds {
		remaining = windowSeconds
	}
	progress := 1 - (remaining / windowSeconds)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return progress
}

// parseUnixMilliString, JSON'da string olarak gelen unix milisaniye
// zaman damgalarını (örn. Cursor'ın "billingCycleEnd" alanı) int64'e çevirir.
func parseUnixMilliString(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ShortDate, bir zamanı yerel saatte "23 Eyl" biçiminde yazar.
func ShortDate(t time.Time) string {
	t = t.In(time.Local)
	return fmt.Sprintf("%d %s", t.Day(), turkishMonths[t.Month()-1])
}

// ShortDateTime, bir zamanı yerel saatte "23 Eki 00:08" biçiminde yazar.
func ShortDateTime(t time.Time) string {
	return ShortDate(t) + " " + t.In(time.Local).Format("15:04")
}

// ClockWithZone, bir zamanı yerel saatte "00:08 GMT+3" biçiminde yazar.
func ClockWithZone(t time.Time) string {
	t = t.In(time.Local)
	_, offset := t.Zone()
	zone := "GMT"
	if h, m := offset/3600, (offset%3600)/60; m != 0 {
		zone += fmt.Sprintf("%+d:%02d", h, abs(m))
	} else if h != 0 {
		zone += fmt.Sprintf("%+d", h)
	}
	return t.Format("15:04") + " " + zone
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
