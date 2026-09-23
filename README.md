# AI Tempo

Claude.ai, Cursor ve ChatGPT hesaplarınızın kota/kullanım limitlerini tek bir
macOS masaüstü uygulamasından takip edin. Her hesap kendi sekmesinde, marka
renginde bir ikonla görünür; her kota kartında **kullanım yüzdesi**,
**sıfırlanmaya kalan süre** ve **tempo** gösterilir.

### Tempo nasıl okunur?

Kullanım çubuğunun üzerindeki **mor dikey çizgi**, dönemin ne kadarının
geçtiğini gösterir ("şu ana kadar harcamış olmanız gereken" nokta).

- Doluluk çizginin **gerisindeyse** → `Tempo: rahat` (kota süreden önce bitmez)
- Çizgiye **yakınsa** (±10 puan) → `Tempo: normal`
- Çizgiyi **geçmişse** → `Tempo: hızlı ⚠` (bu hızla kota erken biter;
  menü çubuğunda da `⚠ hızlı` olarak görünür)

> ⚠️ Bu araç, ilgili servislerin **resmi API'lerini değil**, web
> arayüzlerinin (claude.ai, cursor.com, chatgpt.com) tarayıcıdan giriş
> yaptığınızda kullandığı dahili (public olmayan) uç noktaları kullanır.
> Bu yüzden zaman zaman servisler tarafında değişebilir ve oturum
> anahtarlarının/token'ların süresi dolduğunda yeniden alınması gerekir.
> Sadece kendi hesaplarınız için, kişisel kullanım amacıyla kullanın.

## Kurulum

```bash
make build     # sadece derler -> ./ai-tempo
make run       # derler ve terminalden çalıştırır
make package   # "AI Tempo.app" bundle'ı üretir (Icon.png logosuyla)
open "AI Tempo.app"
```

`.app` **Dock'ta ve Cmd+Tab'da görünmez**; yalnızca menü çubuğunda çalışır.
Bunun için hem `Info.plist`'e `LSUIElement` eklenir hem de açılıştan sonra
kodda etkinleştirme politikası "Accessory" yapılır (`internal/gui/dock_darwin.go`), çünkü
GLFW açılışta uygulamayı normal Dock uygulamasına çeviriyor. `Icon.png` Finder'da ve Cmd+Tab'da uygulama simgesi olarak
kullanılır. Dock'ta görünmesini isterseniz Makefile'daki `LSUIElement`
satırını kaldırıp `make package` çalıştırın. İmzasız (ad-hoc) olduğu için
ilk açılışta Gatekeeper uyarısı çıkarsa Finder'da sağ tık → **Aç** deyin.

### Açılışta Otomatik Başlatma

```bash
make login-add      # oturum açılışına ekler
make login-remove   # kaldırır
```

Öğe, proje klasöründeki `.app`'i gösterir; klasörü taşırsanız
`make login-remove && make login-add` ile yeniden ekleyin. Sistem Ayarları →
Genel → Giriş Öğeleri'nden de görülebilir/kaldırılabilir.

## Menü Çubuğu (Saatin Yanı)

Uygulama açıldığında menü çubuğuna bir gösterge ikonu eklenir. İkona
tıklayınca açılan menüde:

- Her hesap için başlık satırı (tıklanınca pencere o hesabın sekmesinde açılır)
- Altında her kotanın durumu: 🟢/🟠/🔴 yüzde ve sıfırlanmaya kalan süre
- **Tümünü Yenile** (son yenileme saatiyle) — menü ayrıca her 5 dakikada bir
  kendiliğinden yenilenir (Ayarlar → Otomatik yenileme)
- **Pencereyi Göster**, **Ayarlar…** ve **Çıkış**

**Ayarlar…** hesap yönetimi penceresini açar (bkz. aşağıda).

Pencerenin kapatma (✕) düğmesi uygulamayı kapatmaz, sadece gizler. Tamamen
çıkmak için menüdeki **Çıkış**'ı kullanın (veya Cmd+Q).

## Ayarlar ve Hesaplar

Hesaplar uygulama içinden yönetilir: menü çubuğunda **Ayarlar…** veya
penceredeki **⚙ Ayarlar** düğmesi. Buradan:

- Hesap **ekleyebilir, düzenleyebilir, silebilir**, etkin/pasif yapabilirsiniz
- Süresi dolan anahtarı **Düzenle** → yeni anahtarı yapıştırarak yenilersiniz
  (alan boş bırakılırsa mevcut anahtar korunur)
- **Otomatik yenileme** aralığını seçersiniz (5 / 10 / 15 / 30 / 60 dk)

**Nerede saklanır?**

| Ne | Nerede |
|---|---|
| Hesap listesi, isimler, org ID, ayarlar | `~/Library/Application Support/ai-tempo/settings.json` |
| Oturum anahtarları / token'lar | macOS **Anahtar Zinciri** (Keychain), servis adı `ai-tempo` |

Anahtarlar hiçbir zaman diske düz metin olarak yazılmaz.

**Eski addan geçiş:** Uygulamanın eski adı "AI Kota Sorgulama" idi. İlk
açılışta eski ayar klasörü (`ai-kota-sorgulama`) ve Keychain kayıtları
otomatik olarak yeni ada (`ai-tempo`) taşınır, eskileri silinir.

**Eski `config.json`'dan geçiş:** `settings.json` yoksa uygulama ilk açılışta
eski `config.json`'u (proje klasörü, binary'nin yanı veya
`~/.config/ai-kota-sorgulama/`) bulup hesapları otomatik içe aktarır ve
anahtarları Keychain'e taşır. İçe aktarımdan sonra `config.json` artık
kullanılmaz; anahtarlar düz metin durmasın diye **silmeniz önerilir**.

## Sorgu Sıklığı ve Engellenme Riski

Tüm sorgular tek bir yerden (`internal/usage`) yapılır; pencere ve menü çubuğu aynı
sonuçları paylaşır:

- Her hesap, seçilen aralıkta (varsayılan 5 dk) **bir kez** sorgulanır —
  tarayıcıda açık duran bir dashboard sekmesinden daha az istek demektir.
- Hesaplar **sırayla**, aralarında 1,5 sn bekleyerek sorgulanır (aynı anda
  istek yağdırılmaz).
- Hata alan hesabın aralığı her hatada **ikiye katlanır** (en fazla 16×);
  süresi dolmuş bir anahtar servise sürekli istek atmaz.
- Elle yenileme aynı hesap için 30 sn'de bir kez çalışır.

## Key / Token Nasıl Alınır

Üç servis de kendi web sitenizde oturum açmışken tarayıcının **Geliştirici
Araçları → Network** sekmesinden alınır. Genel adımlar:

1. İlgili siteye tarayıcıdan giriş yapın.
2. `F12` (veya Cmd+Opt+I) ile Geliştirici Araçları'nı açın, **Network**
   sekmesine geçin, **XHR/Fetch** filtresini seçin.
3. Aşağıda tarif edilen sayfayı ziyaret edin / yenileyin.
4. İlgili isteği bulup **Copy → Copy as cURL** ile kopyalayın, oradan
   gereken cookie/header değerini alın.

Oturum süresi dolduğunda (uygulama "Session süresi dolmuş" / "Token süresi
dolmuş" hatası verirse) aynı adımları tekrarlayıp **Ayarlar → hesap →
Düzenle** ile yeni anahtarı girin. Aşağıdaki "→ `session_key`" /
"→ `organization_id`" ifadeleri, Ayarlar'daki **Oturum anahtarı** ve
**Organization ID** alanlarına karşılık gelir.

### Claude (`provider: "claude"`)

1. [claude.ai](https://claude.ai) adresine giriş yapın.
2. Herhangi bir sohbet sayfasındayken Network sekmesini açın, bir mesaj
   gönderin veya sayfayı yenileyin.
3. `usage` içeren bir istek arayın:
   `GET https://claude.ai/api/organizations/<ORG_ID>/usage`
4. **Cookie** başlığından `sessionKeyV3` değerini kopyalayın →
   `session_key` alanına yazın.
5. İsteğin URL'sindeki `/organizations/` ile `/usage` arasındaki UUID →
   `organization_id` alanına yazın.

### Cursor (`provider: "cursor"`)

1. [cursor.com/dashboard/spending](https://cursor.com/dashboard/spending)
   sayfasına gidin.
2. Network sekmesinde şu isteği bulun:
   `POST https://cursor.com/api/dashboard/get-current-period-usage`
3. **Cookie** başlığından `WorkosCursorSessionToken` değerini kopyalayın
   (URL-encode edilmiş `%3A%3A` kısmını `::` olarak decode edin) →
   `session_key` alanına yazın.
4. Cursor için `organization_id` gerekmez, boş bırakabilir veya alanı hiç
   eklemeyebilirsiniz.

### ChatGPT (`provider: "chatgpt"`)

1. [chatgpt.com](https://chatgpt.com) adresinde bir sohbet açın (herhangi
   bir sayfa yeterli).
2. Network sekmesinde şu isteği bulun:
   `GET https://chatgpt.com/backend-api/wham/usage`
3. **Request Headers** içindeki `authorization: Bearer <token>` satırından
   `Bearer ` sonrasındaki uzun JWT değerini kopyalayın →
   `session_key` alanına yazın (yalnızca token, `Bearer` kelimesi olmadan).
4. Aynı istek başlıklarındaki `chatgpt-account-id` değerini →
   `organization_id` alanına yazın.

---

## Proje Yapısı

```
.
├── cmd/ai-tempo/main.go     # giriş noktası (yalnızca gui.Run() çağırır)
├── internal/
│   ├── config/              # ayarlar (settings.json), Keychain, eski config içe aktarımı
│   ├── provider/            # Claude / Cursor / ChatGPT sorguları, tempo hesabı, tarih yardımcıları
│   ├── usage/               # merkezi sorgu zamanlayıcı (aralık, sıralı istek, hata geri çekilmesi)
│   └── gui/                 # pencere, menü çubuğu, ayarlar penceresi, tema, ikonlar
│       └── icons/           # gömülü SVG/PNG ikonlar
├── assets/Icon.png          # .app / Finder simgesi
├── Makefile
└── go.mod
```

| Paket | Sorumluluk |
|---|---|
| `config` | `Account`/`Settings` tipleri, `settings.json` okuma/yazma, Keychain erişimi, eski addan ve `config.json`'dan geçiş |
| `provider` | `Provider` arayüzü, her servis için ayrı dosya (`claude.go`, `cursor.go`, `chatgpt.go`), `CalcPace` |
| `usage` | Tüm sorguları yapan `Store`; pencere ve menü aynı sonuçları paylaşır. GUI'ye bağımlı değildir |
| `gui` | `app.go` (açılış, sekmeler), `tray.go`, `settings_window.go`, `cards.go`, `colorbar.go`, `theme.go`, `dock_darwin.go` |

Bağımlılık yönü tek yönlüdür: `gui → usage → provider → config`.

Yeni bir sağlayıcı eklemek için `internal/provider/` altına `Provider` arayüzünü
implemente eden yeni bir dosya (örn. `gemini.go`) yazıp `init()` içinde
`register(...)` ile kaydetmeniz yeterlidir; ikonunu `internal/gui/icons/`'a
ekleyip `internal/gui/cards.go` → `providerIcon` içine tanımlayın.
