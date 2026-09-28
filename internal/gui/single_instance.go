package gui

import (
	"bufio"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"ai-tempo/internal/config"
)

// instanceSocket, çalışan kopyanın dinlediği Unix soketidir.
func instanceSocket() string { return filepath.Join(config.DataDir(), "instance.sock") }

// claimSingleInstance, uygulamanın tek kopya çalışmasını sağlar. Başka bir kopya
// zaten çalışıyorsa ona "show" mesajı gönderip false döner (çağıran çıkmalıdır).
// İlk kopyada true döner; onShow, sonradan açılmaya çalışılan her kopya için çağrılır.
func claimSingleInstance(onShow func()) bool {
	path := instanceSocket()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)

	ln, err := net.Listen("unix", path)
	if err != nil && errors.Is(err, syscall.EADDRINUSE) {
		// Soket var: gerçekten dinleyen bir kopya mı, yoksa çökmüş bir kopyadan mı kalma?
		if c, dialErr := net.DialTimeout("unix", path, time.Second); dialErr == nil {
			_, _ = c.Write([]byte("show\n"))
			c.Close()
			return false
		}
		_ = os.Remove(path)
		ln, err = net.Listen("unix", path)
	}
	if err != nil {
		// Soket açılamadıysa tek kopya denetimi yapılamaz; uygulama yine çalışsın.
		return true
	}

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(c).ReadString('\n')
			c.Close()
			if line == "show\n" {
				onShow()
			}
		}
	}()
	return true
}

// releaseSingleInstance, çıkışta soket dosyasını kaldırır.
func releaseSingleInstance() { _ = os.Remove(instanceSocket()) }
