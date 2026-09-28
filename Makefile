APP_NAME     := ai-tempo
DISPLAY_NAME := AI Tempo
APP_BUNDLE   := $(DISPLAY_NAME).app
APP_ID       := com.karakunt.ai-tempo
GOBIN        := $(shell go env GOPATH)/bin
FYNE         := $(GOBIN)/fyne

# İmza / onay (notarization). Değerler depoya yazılmaz; yerel Keychain'den gelir.
SIGN_IDENTITY  ?= $(shell security find-identity -v -p codesigning | grep -m1 "Developer ID Application" | awk '{print $$2}')
NOTARY_PROFILE ?= ai-tempo
NOTARIZE       ?= 1
WIN_CC         ?= x86_64-w64-mingw32-gcc

# Sürüm: son git etiketinden (v1.2.3 → 1.2.3). Yeni sürümde: make release VERSION=1.2.4
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
LDFLAGS := -X ai-tempo/internal/gui.Version=$(VERSION)

.PHONY: help build run clean tidy fmt package release release-windows

## help: Kullanılabilir komutları listeler (varsayılan hedef)
help:
	@echo "Kullanılabilir komutlar:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/^## /  /'

## build: Derler ve ./$(APP_NAME) binary'sini üretir
build:
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) ./cmd/ai-tempo

## run: Derler ve uygulamayı çalıştırır
run: build
	./$(APP_NAME)

## clean: Derleme çıktılarını temizler
clean:
	rm -f $(APP_NAME)
	rm -rf "$(APP_BUNDLE)" dist

## tidy: go.mod / go.sum dosyalarını günceller
tidy:
	go mod tidy

## fmt: Kod formatlar
fmt:
	go fmt ./...

## package: macOS için .app bundle oluşturur. LSUIElement sayesinde Dock'ta
## görünmez, yalnızca menü çubuğunda (saatin yanında) çalışır. fyne CLI eksikse otomatik kurulur
## (go install fyne.io/fyne/v2/cmd/fyne@latest).
package: $(FYNE)
	rm -rf "$(APP_BUNDLE)"
	$(FYNE) package -os darwin -name "$(DISPLAY_NAME)" -appID $(APP_ID) -appVersion "$(VERSION)" -icon "$(CURDIR)/assets/Icon.png" -src ./cmd/ai-tempo
	/usr/libexec/PlistBuddy -c "Add :LSUIElement bool true" "$(APP_BUNDLE)/Contents/Info.plist"
	codesign --force --deep -s - "$(APP_BUNDLE)"
	@echo ""
	@echo "✓ $(APP_BUNDLE) hazır. İlk açılışta Gatekeeper uyarısı çıkarsa"
	@echo "  Finder'da uygulamaya sağ tıklayıp 'Aç' deyin."

## release: Dağıtım için Intel + Apple Silicon (universal) .app üretir, Developer ID
## ile imzalar, Apple'a onaylatır (notarize) ve dist/AI-Tempo-macOS.zip olarak paketler.
## Kimlik bilgileri depoya yazılmaz: imza sertifikası Keychain'den bulunur, notary
## bilgileri `xcrun notarytool store-credentials $(NOTARY_PROFILE)` ile bir kez kaydedilir.
## NOTARIZE=0 ile yalnızca imzalar (onaysız).
release: package
	CGO_ENABLED=1 GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-amd64 ./cmd/ai-tempo
	CGO_ENABLED=1 GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)-arm64 ./cmd/ai-tempo
	lipo -create -output "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)" dist/$(APP_NAME)-amd64 dist/$(APP_NAME)-arm64
	rm -f dist/$(APP_NAME)-amd64 dist/$(APP_NAME)-arm64
	@test -n "$(SIGN_IDENTITY)" || { echo "❌ Keychain'de 'Developer ID Application' sertifikası yok"; exit 1; }
	codesign --force --deep --options runtime --timestamp -s "$(SIGN_IDENTITY)" "$(APP_BUNDLE)"
	codesign --verify --strict --deep "$(APP_BUNDLE)"
	rm -f dist/AI-Tempo-macOS.zip
ifneq ($(NOTARIZE),0)
	ditto -c -k --keepParent "$(APP_BUNDLE)" dist/notarize.zip
	xcrun notarytool submit dist/notarize.zip --keychain-profile "$(NOTARY_PROFILE)" --wait
	rm -f dist/notarize.zip
	xcrun stapler staple "$(APP_BUNDLE)"
	spctl --assess --type execute "$(APP_BUNDLE)"
endif
	ditto -c -k --keepParent "$(APP_BUNDLE)" dist/AI-Tempo-macOS.zip
	@echo "✓ dist/AI-Tempo-macOS.zip hazır ($$(lipo -archs "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)"))"

## release-windows: Windows (64-bit) sürümünü bu Mac'ten çapraz derler ve
## dist/AI-Tempo-Windows.zip olarak paketler. mingw-w64 gerekir (brew install mingw-w64).
## Exe imzasızdır; Windows ilk açılışta SmartScreen uyarısı gösterebilir.
release-windows: $(FYNE)
	@command -v $(WIN_CC) >/dev/null || { echo "❌ $(WIN_CC) yok: brew install mingw-w64"; exit 1; }
	CGO_ENABLED=1 CC=$(WIN_CC) GOOS=windows GOARCH=amd64 $(FYNE) package -os windows -name "$(DISPLAY_NAME)" -appID $(APP_ID) -appVersion "$(VERSION)" -icon "$(CURDIR)/assets/Icon.png" -src ./cmd/ai-tempo
	mkdir -p dist
	mv "cmd/ai-tempo/$(DISPLAY_NAME).exe" "dist/$(DISPLAY_NAME).exe"
	rm -f dist/AI-Tempo-Windows.zip
	cd dist && zip -q AI-Tempo-Windows.zip "$(DISPLAY_NAME).exe" && rm "$(DISPLAY_NAME).exe"
	@echo "✓ dist/AI-Tempo-Windows.zip hazır"

$(FYNE):
	@echo "fyne CLI bulunamadı, kuruluyor..."
	go install fyne.io/fyne/v2/cmd/fyne@latest
