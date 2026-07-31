//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestDriverInstallProbe(t *testing.T) {
	if os.Getenv("VSC_DRIVER_INSTALL_PROBE") != "1" {
		t.Skip("set VSC_DRIVER_INSTALL_PROBE=1 for the elevated installation probe")
	}
	packageDirectory := os.Getenv("VSC_DRIVER_PACKAGE")
	if err := validateDriverPackage(packageDirectory); err != nil {
		t.Fatal(err)
	}
	logger := log.New(os.Stdout, "driver-probe: ", log.LstdFlags|log.Lmicroseconds)
	executable, err := installBundledUSBIP(packageDirectory, logger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runUSBIP(executable, logger, "port"); err != nil {
		t.Fatalf("installed driver is not operational: %v", err)
	}
	t.Logf("driver installation and usbip port check passed: %s", executable)
}

func TestValidateDriverPackage(t *testing.T) {
	directory := t.TempDir()
	files := append(append([]string{}, usbipDriverFiles...), prefixedRuntimeFiles()...)
	for _, relative := range files {
		path := filepath.Join(directory, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateDriverPackage(directory); err != nil {
		t.Fatalf("complete package rejected: %v", err)
	}
	if err := os.Remove(filepath.Join(directory, usbipDriverFiles[0])); err != nil {
		t.Fatal(err)
	}
	if err := validateDriverPackage(directory); err == nil {
		t.Fatal("incomplete package was accepted")
	}
}

func TestPathContainsDirectory(t *testing.T) {
	pathValue := `C:\Windows;"C:\Program Files\Virtual Surround Cables\USBip";C:\Tools`
	if !pathContainsDirectory(pathValue, `c:\program files\virtual surround cables\usbip\`) {
		t.Fatal("expected case-insensitive normalized path match")
	}
	if pathContainsDirectory(pathValue, `C:\Program Files\Other`) {
		t.Fatal("unexpected path match")
	}
}
