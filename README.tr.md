# AI Tempo

[English](README.md) · **Türkçe**

**Claude.ai**, **Cursor** ve **ChatGPT** hesaplarınızın kullanım limitlerini
tek yerden takip eden bir macOS menü çubuğu uygulaması. Her hesap kendi
sekmesinde görünür. Her kota kartında **kullanım yüzdesi**, **sıfırlanmaya
kalan süre** ve **tempo** gösterilir. Tempo, bu hızla kotanın dönem bitmeden
dolup dolmayacağını söyler.

Arayüz **İngilizce** ve **Türkçe** kullanılabilir. Varsayılan olarak macOS
dilinizi izler: sistem Türkçeyse Türkçe, değilse İngilizce açılır. Dili
Ayarlar → Dil bölümünden değiştirebilirsiniz.

> ⚠️ AI Tempo **resmi API'leri kullanmaz**. claude.ai, cursor.com ve
> chatgpt.com'un tarayıcıda kullandığı dahili uç noktaları çağırır. Bu uç
> noktalar haber verilmeden değişebilir. Oturum anahtarlarının ve
> token'ların süresi dolunca yenilenmeleri gerekir. Yalnızca kendi
> hesaplarınız için kullanın.

## İndirme

1. [Son sürümden](https://github.com/KaraKunT/AI-Tempo/releases/latest)
   **`AI-Tempo-macOS.zip`** dosyasını indirin. Apple Silicon ve Intel
   Mac'lerde çalışır.
2. Zip'i açın ve **AI Tempo.app**'i Uygulamalar klasörüne taşıyın.

Uygulama Developer ID ile imzalı ve Apple tarafından onaylı (notarize
edilmiş). Güvenlik uyarısı olmadan açılır.

AI Tempo yalnızca menü çubuğunda çalışır. Dock'ta ve Cmd+Tab'da görünmez.

## Özellikler

- **Menü çubuğu özeti.** Her kota 🟢/🟠/🔴 durumu, yüzdesi ve sıfırlanmaya
  kalan süresiyle listelenir. Bir hesaba tıklayınca o hesabın sekmesi açılır.
- **Tempo.** Kullanım çubuğundaki mor çizgi, dönemin ne kadarının geçtiğini
  gösterir.
  - `Tempo: rahat`: kullanım çizginin gerisinde.
  - `Tempo: normal`: kullanım çizgiye ±10 puan yakın.
  - `Tempo: hızlı ⚠`: kullanım çizginin önünde, kota erken bitebilir.
- **Dönem grafiği.** Dönemin başından reset tarihine kadar kullanım, "ideal
  tempo" çizgisiyle birlikte çizilir. Dönem / Bugün / Dün / Hafta / Ay
  filtreleri var.
- **Sorgu geçmişi.** Her sorgunun saati, tetikleyeni (Otomatik / Elle /
  Sıfırlama), sonucu ve süresi tutulur. Bugün / Dün / Hafta / Ay filtreleri
  var.
- **Sekme başlığında durum.** `⟳` hesabın o an sorgulandığını, `⚠` son
  sorgusunun hata verdiğini gösterir.
- **Deneme sınırı.** Üst üste 5 hatadan sonra hesap otomatik sorgulanmaz.
  Hesabın sekmesindeki **Sayacı Sıfırla** ile yeniden açılır.
- **Codex sıfırlama hakları** (ChatGPT, isteğe bağlı). Kullanılabilir
  ücretsiz sıfırlama haklarını ve alınan/kullanılan hakların geçmişini
  gösterir.
- Ana pencere ve Ayarlar penceresi bıraktığınız konumda ve boyutta açılır.

## Hesap ekleme

Menü çubuğundan ya da pencereden **Ayarlar**'ı açın ve **Hesap Ekle**'ye
tıklayın. Her sağlayıcı için, siteye giriş yapmışken tarayıcıdan
kopyaladığınız bir oturum anahtarı (bazen bir de ID) gerekir.

Hesap düzenleyicideki **Konsoldan al** düğmesi, tarayıcının Geliştirici
Araçları konsoluna yapıştıracağınız bir kod verir. Bu kod değerleri tam
haliyle yazdırır. Network sekmesinden kopyalanan değerler `…` ile sessizce
kesilebilir. Kesilmiş anahtar da "süresi dolmuş" hatası verir.

| Sağlayıcı | Oturum anahtarı | Organization ID |
|---|---|---|
| **Claude** | `sessionKeyV3` cookie'si. Geliştirici Araçları → **Application → Cookies → claude.ai** | claude.ai'de konsol kodunu çalıştırın ya da `…/usage` isteğinin URL'sinde `/organizations/` ile `/usage` arasındaki UUID'yi kopyalayın |
| **Cursor** | `WorkosCursorSessionToken` cookie'si. **Application → Cookies → cursor.com** (`%3A%3A` otomatik olarak `::`'ya çevrilir) | Gerekmez |
| **ChatGPT** | chatgpt.com'da konsol kodunu çalıştırın (token panoya kopyalanır) ya da `backend-api/wham/usage` isteğindeki `Bearer` token'ı kopyalayın | Aynı istekteki `chatgpt-account-id` başlığı (konsol kodu bunu da yazdırır) |

Claude ve Cursor cookie'lerini HttpOnly olarak işaretler, bu yüzden
JavaScript bunları genelde okuyamaz. Bu iki sağlayıcının anahtarını
**Application → Cookies** panelinden kopyalayın.

Anahtarın süresi dolunca aynı adımları tekrarlayın ve yeni anahtarı
**Ayarlar → hesap → Düzenle** bölümüne yapıştırın. Alanı boş bırakırsanız
mevcut anahtar korunur.

## Ayarlar

| Ayar | Seçenekler |
|---|---|
| Dil | Otomatik (sistem dili, varsayılan), English, Türkçe |
| Otomatik yenileme | 5, 10, 15, 30 dakika · 1, 2, 4, 8, 16, 24 saat |
| Geçmişi sakla | 2, 7, 14, 35 (varsayılan), 60, 90 gün. Cursor gibi aylık dönemler için en az 35 gün seçin |

### Veriler nerede saklanır?

| Ne | Nerede |
|---|---|
| Hesaplar, ID'ler, ayarlar | `~/Library/Application Support/ai-tempo/settings.json` |
| Oturum anahtarları / token'lar | macOS **Anahtar Zinciri** (Keychain), servis adı `ai-tempo`. Diske asla düz metin olarak yazılmaz |
| Sorgu geçmişi ve grafik verisi | `~/Library/Application Support/ai-tempo/history.db` (SQLite) |

Bütün veriler Mac'inizde kalır. AI Tempo yalnızca claude.ai, cursor.com ve
chatgpt.com ile konuşur.

## Sorgu politikası

Bütün sorgular tek bir zamanlayıcıdan (`internal/usage`) yapılır. Pencere ve
menü çubuğu aynı sonuçları paylaşır:

- Her hesap, yenileme aralığında **bir kez** sorgulanır. Bu, tarayıcıda açık
  duran bir dashboard sekmesinin attığından daha az istek demektir.
- Hesaplar **sırayla**, aralarında 1,5 sn bekleyerek sorgulanır.
- Sorunsuz bir hesabı elle yenileme en fazla 30 sn'de bir çalışır. Hata
  veren hesap hemen yeniden denenebilir.
- Claude, Cloudflare'in arkasında. Cloudflare tarayıcı dışı istekleri bazen
  HTTP 403 ile engelliyor. AI Tempo bunu "süresi dolmuş" olarak değil,
  "Cloudflare engeli" olarak gösterir ve yeni bir bağlantıyla bir kez daha
  dener.

## Kaynaktan derleme

Go ve Xcode Command Line Tools gerekir.

```bash
make run       # derler ve terminalden çalıştırır
make package   # "AI Tempo.app" üretir
make release   # Intel + Apple Silicon (universal), imzalı + onaylı → dist/AI-Tempo-macOS.zip
make release NOTARIZE=0   # yalnızca imzalar (Developer ID sertifikası gerekir)
```

Oturum açılışında başlatma:

```bash
make login-add      # bu klasördeki .app'i giriş öğelerine ekler
make login-remove
```

## Proje yapısı

```
cmd/ai-tempo/        giriş noktası
internal/config/     settings.json, Keychain, geçişler
internal/provider/   Claude / Cursor / ChatGPT sorguları, tempo, tarih yardımcıları
internal/usage/      merkezi sorgu zamanlayıcı
internal/history/    SQLite sorgu geçmişi ve grafik ölçümleri
internal/i18n/       çeviriler (Türkçe kaynak metinler → İngilizce)
internal/gui/        pencere, menü çubuğu, ayarlar, grafikler, tema, ikonlar
```

Bağımlılıklar tek yönlüdür: `gui → usage → provider → config`.

Yeni bir sağlayıcı eklemek için `internal/provider/` altında `Provider`
arayüzünü uygulayan bir dosya yazın ve `init()` içinde kaydedin. İkonunu
`internal/gui/icons/` klasörüne ve `providerIcon` fonksiyonuna ekleyin.
Arayüz metinleri Türkçe yazılır ve `T(...)` ile sarılır. İngilizce
karşılığını `internal/i18n/en.go` dosyasına ekleyin.

## Lisans

[MIT](LICENSE)
