APP_NAME     := ai-tempo
DISPLAY_NAME := AI Tempo
APP_BUNDLE   := $(DISPLAY_NAME).app
APP_ID       := com.karakunt.ai-tempo
GOBIN        := $(shell go env GOPATH)/bin
FYNE         := $(GOBIN)/fyne

.PHONY: help build run clean tidy fmt package release login-add login-remove

## help: Kullanılabilir komutları listeler (varsayılan hedef)
help:
	@echo "Kullanılabilir komutlar:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/^## /  /'

## build: Derler ve ./$(APP_NAME) binary'sini üretir
build:
	go build -o $(APP_NAME) ./cmd/ai-tempo

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
	$(FYNE) package -os darwin -name "$(DISPLAY_NAME)" -appID $(APP_ID) -icon "$(CURDIR)/assets/Icon.png" -src ./cmd/ai-tempo
	/usr/libexec/PlistBuddy -c "Add :LSUIElement bool true" "$(APP_BUNDLE)/Contents/Info.plist"
	codesign --force --deep -s - "$(APP_BUNDLE)"
	@echo ""
	@echo "✓ $(APP_BUNDLE) hazır. İlk açılışta Gatekeeper uyarısı çıkarsa"
	@echo "  Finder'da uygulamaya sağ tıklayıp 'Aç' deyin."

## release: Dağıtım için Intel + Apple Silicon (universal) .app üretir ve
## dist/AI-Tempo-macOS.zip olarak paketler (GitHub Releases'e yüklenecek dosya).
release: package
	CGO_ENABLED=1 GOARCH=amd64 go build -o dist/$(APP_NAME)-amd64 ./cmd/ai-tempo
	CGO_ENABLED=1 GOARCH=arm64 go build -o dist/$(APP_NAME)-arm64 ./cmd/ai-tempo
	lipo -create -output "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)" dist/$(APP_NAME)-amd64 dist/$(APP_NAME)-arm64
	rm -f dist/$(APP_NAME)-amd64 dist/$(APP_NAME)-arm64
	codesign --force --deep -s - "$(APP_BUNDLE)"
	ditto -c -k --keepParent "$(APP_BUNDLE)" dist/AI-Tempo-macOS.zip
	@echo "✓ dist/AI-Tempo-macOS.zip hazır ($$(lipo -archs "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)"))"

## login-add: .app'i oturum açılışında otomatik başlatılacak şekilde ekler
login-add:
	osascript -e 'tell application "System Events" to make login item at end with properties {path:"$(CURDIR)/$(APP_BUNDLE)", hidden:true}'

## login-remove: .app'i oturum açılış öğelerinden kaldırır
login-remove:
	osascript -e 'tell application "System Events" to delete login item "$(DISPLAY_NAME)"'

$(FYNE):
	@echo "fyne CLI bulunamadı, kuruluyor..."
	go install fyne.io/fyne/v2/cmd/fyne@latest
