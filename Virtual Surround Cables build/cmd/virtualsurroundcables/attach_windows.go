//go:build windows

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var usbipPortPattern = regexp.MustCompile(`(?i)^Port\s+(\d+):`)

const usbipDeviceHost = "127.0.0.3"

var (
	advapi32               = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx       = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx    = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueEx      = advapi32.NewProc("RegSetValueExW")
	procRegCloseKey        = advapi32.NewProc("RegCloseKey")
	procSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
)

const (
	hkeyLocalMachine = uintptr(0x80000002)
	keyQueryValue    = 0x0001
	keySetValue      = 0x0002
	regExpandSZ      = 2
	wmSettingChange  = 0x001A
	hwndBroadcast    = 0xFFFF
	smtoAbortIfHung  = 0x0002
)

var usbipRuntimeFiles = []string{
	"usbip.exe", "devnode.exe", "libusbip.dll", "resources.dll",
	"MSVCP140.dll", "VCRUNTIME140.dll", "VCRUNTIME140_1.dll",
}

var usbipDriverFiles = []string{
	filepath.Join("drivers", "filter", "usbip2_filter.inf"),
	filepath.Join("drivers", "filter", "usbip2_filter.cat"),
	filepath.Join("drivers", "filter", "usbip2_filter.sys"),
	filepath.Join("drivers", "ude", "usbip2_ude.inf"),
	filepath.Join("drivers", "ude", "usbip2_ude.cat"),
	filepath.Join("drivers", "ude", "usbip2_ude.sys"),
}

func findUSBIPExecutable() string {
	if path, err := exec.LookPath("usbip.exe"); err == nil {
		return path
	}
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		for _, folder := range []string{filepath.Join("Virtual Surround Cables", "USBip"), filepath.Join("Virtual Cables Pro", "USBip"), "USBip", "usbip-win2", "usbip"} {
			candidate := filepath.Join(root, folder, "usbip.exe")
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func runUSBIP(executable string, logger *log.Logger, arguments ...string) (string, error) {
	logger.Printf("usbip.exe %s", strings.Join(arguments, " "))
	command := exec.Command(executable, arguments...)
	hideCommandWindow(command)
	output, err := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if text != "" {
		logger.Printf("usbip: %s", strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", " | "), "\n", " | "))
	}
	if err != nil {
		return text, fmt.Errorf("usbip %s: %w", strings.Join(arguments, " "), err)
	}
	return text, nil
}

func hideCommandWindow(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

func bundledDriverPackage() string {
	exeDir := filepath.Dir(executablePath())
	candidates := []string{
		filepath.Join(exeDir, "Driver", "USBip"),
		filepath.Join(exeDir, "assets", "driver", "USBip"),
		filepath.Join(filepath.Dir(filepath.Dir(exeDir)), "assets", "driver", "USBip"),
	}
	for _, candidate := range candidates {
		if validateDriverPackage(candidate) == nil {
			return candidate
		}
	}
	return ""
}

func prefixedRuntimeFiles() []string {
	files := make([]string, 0, len(usbipRuntimeFiles))
	for _, name := range usbipRuntimeFiles {
		files = append(files, filepath.Join("bin", name))
	}
	return files
}

func validateDriverPackage(directory string) error {
	for _, relative := range append(append([]string{}, usbipDriverFiles...), prefixedRuntimeFiles()...) {
		path := filepath.Join(directory, relative)
		if info, err := os.Stat(path); err != nil || info.IsDir() || info.Size() == 0 {
			return fmt.Errorf("required driver file is unavailable: %s", path)
		}
	}
	return nil
}

func installedUSBIPDirectory() string {
	root := os.Getenv("ProgramFiles")
	if strings.TrimSpace(root) == "" {
		root = filepath.Join(os.Getenv("SystemDrive")+string(os.PathSeparator), "Program Files")
	}
	return filepath.Join(root, "Virtual Surround Cables", "USBip")
}

func copyBundledUSBIPRuntime(packageDirectory string) (string, error) {
	destination := installedUSBIPDirectory()
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", fmt.Errorf("create USB/IP program directory: %w", err)
	}
	for _, name := range usbipRuntimeFiles {
		source := filepath.Join(packageDirectory, "bin", name)
		data, err := os.ReadFile(source)
		if err != nil {
			return "", fmt.Errorf("read bundled %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(destination, name), data, 0o644); err != nil {
			return "", fmt.Errorf("install %s: %w", name, err)
		}
	}
	return filepath.Join(destination, "usbip.exe"), nil
}

func runDriverCommand(logger *log.Logger, executable string, arguments ...string) error {
	command := exec.Command(executable, arguments...)
	hideCommandWindow(command)
	output, err := command.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if text != "" {
		logger.Printf("%s %s: %s", filepath.Base(executable), strings.Join(arguments, " "), strings.ReplaceAll(text, "\r\n", " | "))
	}
	if err != nil {
		return fmt.Errorf("%s failed: %w", filepath.Base(executable), err)
	}
	return nil
}

func commandExitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}

func installBundledUSBIP(packageDirectory string, logger *log.Logger) (string, error) {
	executable, err := copyBundledUSBIPRuntime(packageDirectory)
	if err != nil {
		return "", err
	}
	systemRoot := os.Getenv("SystemRoot")
	if strings.TrimSpace(systemRoot) == "" {
		systemRoot = `C:\Windows`
	}
	pnputil := filepath.Join(systemRoot, "System32", "pnputil.exe")
	filterINF := filepath.Join(packageDirectory, "drivers", "filter", "usbip2_filter.inf")
	udeINF := filepath.Join(packageDirectory, "drivers", "ude", "usbip2_ude.inf")
	if err := runDriverCommand(logger, pnputil, "/add-driver", filterINF, "/install"); err != nil {
		if commandExitCode(err) != 259 {
			return "", fmt.Errorf("install USB/IP filter driver: %w", err)
		}
		logger.Printf("The signed filter driver is already present and current; continuing")
	}
	if _, readyErr := runUSBIP(executable, logger, "port"); readyErr == nil {
		if pathErr := ensureMachinePath(filepath.Dir(executable)); pathErr != nil {
			logger.Printf("Could not add USB/IP tools to the machine PATH: %v", pathErr)
		}
		logger.Printf("USB/IP virtual host controller is already operational")
		return executable, nil
	}
	devnode := filepath.Join(filepath.Dir(executable), "devnode.exe")
	if err := runDriverCommand(logger, devnode, "install", udeINF, `ROOT\USBIP_WIN2\UDE`); err != nil {
		return "", fmt.Errorf("install USB/IP virtual host controller: %w", err)
	}
	if err := ensureMachinePath(filepath.Dir(executable)); err != nil {
		logger.Printf("Could not add USB/IP tools to the machine PATH: %v", err)
	}
	return executable, nil
}

func ensureUSBIPDriver(logger *log.Logger) (string, error) {
	if executable := findUSBIPExecutable(); executable != "" {
		if _, err := runUSBIP(executable, logger, "port"); err == nil {
			if pathErr := ensureMachinePath(filepath.Dir(executable)); pathErr != nil {
				logger.Printf("Could not add existing USB/IP tools to the machine PATH: %v", pathErr)
			}
			return executable, nil
		}
		logger.Printf("USB/IP tools exist but the driver is not ready; repairing from the bundled package")
	}
	packageDirectory := bundledDriverPackage()
	if packageDirectory == "" {
		return "", fmt.Errorf("the bundled usbip-win2 driver package was not found or is incomplete")
	}
	executable, err := installBundledUSBIP(packageDirectory, logger)
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < 20; attempt++ {
		if _, readyErr := runUSBIP(executable, logger, "port"); readyErr == nil {
			return executable, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("usbip-win2 was installed but is not ready; restart Windows and open Virtual Surround Cables again")
}

func ensureMachinePath(directory string) error {
	directory = filepath.Clean(directory)
	keyPath := utf16(`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`)
	var key uintptr
	result, _, _ := procRegOpenKeyEx.Call(hkeyLocalMachine, uintptr(unsafe.Pointer(keyPath)), 0, keyQueryValue|keySetValue, uintptr(unsafe.Pointer(&key)))
	if result != 0 {
		return syscall.Errno(result)
	}
	defer procRegCloseKey.Call(key)
	valueName := utf16("Path")
	var valueType, byteCount uint32
	result, _, _ = procRegQueryValueEx.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&valueType)), 0, uintptr(unsafe.Pointer(&byteCount)))
	if result != 0 {
		return syscall.Errno(result)
	}
	buffer := make([]uint16, int(byteCount/2)+1)
	result, _, _ = procRegQueryValueEx.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&valueType)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&byteCount)))
	if result != 0 {
		return syscall.Errno(result)
	}
	current := syscall.UTF16ToString(buffer)
	if pathContainsDirectory(current, directory) {
		_ = os.Setenv("PATH", directory+";"+os.Getenv("PATH"))
		return nil
	}
	next := strings.TrimRight(current, " ;")
	if next != "" {
		next += ";"
	}
	next += directory
	encoded, err := syscall.UTF16FromString(next)
	if err != nil {
		return err
	}
	result, _, _ = procRegSetValueEx.Call(key, uintptr(unsafe.Pointer(valueName)), 0, regExpandSZ, uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)*2))
	if result != 0 {
		return syscall.Errno(result)
	}
	_ = os.Setenv("PATH", directory+";"+os.Getenv("PATH"))
	var ignored uintptr
	procSendMessageTimeout.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(utf16("Environment"))), smtoAbortIfHung, 5000, uintptr(unsafe.Pointer(&ignored)))
	return nil
}

func pathContainsDirectory(pathValue, directory string) bool {
	directory = filepath.Clean(directory)
	for _, entry := range strings.Split(pathValue, ";") {
		entry = strings.Trim(strings.TrimSpace(entry), `"`)
		if entry != "" && strings.EqualFold(filepath.Clean(entry), directory) {
			return true
		}
	}
	return false
}

func detachOwnUSBIPPorts(executable string, logger *log.Logger) {
	output, err := runUSBIP(executable, logger, "port")
	if err != nil {
		logger.Printf("Could not inspect existing USB/IP ports: %v", err)
		return
	}
	currentPort := ""
	var ownPorts []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := usbipPortPattern.FindStringSubmatch(trimmed); len(match) == 2 {
			currentPort = match[1]
			continue
		}
		if currentPort != "" && strings.Contains(trimmed, "usbip://"+usbipDeviceHost+":3240/") {
			ownPorts = append(ownPorts, currentPort)
			currentPort = ""
		}
	}
	for _, port := range ownPorts {
		if _, err := runUSBIP(executable, logger, "detach", "-p", port); err != nil {
			logger.Printf("Could not detach previous Virtual Surround Cables port %s: %v", port, err)
		}
	}
}

func runAttachHelper(count int, logger *log.Logger) error {
	if count < 1 || count > 30 {
		return fmt.Errorf("invalid USB device count %d", count)
	}
	executable, err := ensureUSBIPDriver(logger)
	if err != nil {
		return err
	}
	logger.Printf("Elevated Windows device synchronization started for %d multi-cable USB device(s)", count)
	logger.Printf("usbip.exe: %s", executable)

	// The server can still be completing its restart when the elevated helper
	// starts. Retry only the local device-list request for a short bounded time.
	var listErr error
	for attempt := 1; attempt <= 20; attempt++ {
		if _, listErr = runUSBIP(executable, logger, "list", "-r", usbipDeviceHost); listErr == nil {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if listErr != nil {
		return fmt.Errorf("local USB/IP server is not ready: %w", listErr)
	}

	detachOwnUSBIPPorts(executable, logger)
	for number := 1; number <= count; number++ {
		busID := "1-" + strconv.Itoa(number)
		if _, err := runUSBIP(executable, logger, "attach", "-r", usbipDeviceHost, "-b", busID); err != nil {
			return fmt.Errorf("attach %s failed: %w", busID, err)
		}
	}
	logger.Printf("Windows device synchronization completed for %d multi-cable USB device(s)", count)
	return nil
}
