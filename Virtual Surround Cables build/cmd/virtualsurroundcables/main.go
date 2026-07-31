package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"virtualsurroundcables/internal/surround"
	"virtualsurroundcables/internal/usbip"
)

var version = "1.1.0"

type appServer struct {
	mu        sync.Mutex
	cancel    context.CancelFunc
	done      chan error
	count     int
	address   string
	bufferMS  int
	logger    *log.Logger
	logPath   string
	lastError string
}

func (s *appServer) Restart(count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopLocked()

	devices, err := makeDevices(count, s.bufferMS)
	if err != nil {
		s.setErrorLocked(err)
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := usbip.NewServer(s.address, devices, s.logger)
	result := make(chan error, 1)
	go func() { result <- server.Serve(ctx) }()

	select {
	case <-server.Ready():
		s.cancel = cancel
		s.done = result
		s.count = count
		s.lastError = ""
		s.logger.Printf("Virtual Surround Cables %s started with %d logical cable(s) across %d USB device(s)", version, count, len(devices))
		return nil
	case err := <-result:
		cancel()
		if err == nil {
			err = fmt.Errorf("USB/IP server stopped before becoming ready")
		}
		s.setErrorLocked(err)
		return err
	case <-time.After(3 * time.Second):
		cancel()
		err := fmt.Errorf("timed out while opening USB/IP port %s", s.address)
		s.setErrorLocked(err)
		return err
	}
}

func (s *appServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *appServer) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

func (s *appServer) LogPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logPath
}

func (s *appServer) setErrorLocked(err error) {
	if err == nil {
		s.lastError = ""
		return
	}
	s.lastError = err.Error()
	if s.logger != nil {
		s.logger.Printf("USB/IP server error: %v", err)
	}
}

func (s *appServer) stopLocked() {
	if s.cancel == nil {
		return
	}
	s.cancel()
	if s.done != nil {
		select {
		case err := <-s.done:
			if err != nil && s.logger != nil {
				s.logger.Printf("USB/IP server stopped with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			if s.logger != nil {
				s.logger.Printf("USB/IP server shutdown timed out; continuing restart")
			}
		}
	}
	s.cancel = nil
	s.done = nil
}

func main() {
	logger, logFile, logPath := newApplicationLogger()
	if logFile != nil {
		defer logFile.Close()
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("Unexpected startup failure: %v", recovered)
			logger.Printf("PANIC: %s", message)
			showFatalDialog(message + "\n\nDiagnostic log:\n" + logPath)
		}
	}()

	logger.Printf("============================================================")
	logger.Printf("Virtual Surround Cables %s (%s/%s)", version, runtime.GOOS, runtime.GOARCH)
	logger.Printf("Process started. Executable: %s", executablePath())
	logger.Printf("Log file: %s", logPath)
	logger.Printf("USB/IP protocol: 0x%04X", usbip.ProtocolVersion)

	address := flag.String("listen", "127.0.0.3:3240", "USB/IP listen address")
	cables := flag.Int("cables", loadCableCount(), "number of logical 7.1 surround cables (1-120)")
	latencyMS := flag.Int("buffer-ms", 250, "audio ring-buffer capacity in milliseconds")
	selfTest := flag.Bool("self-test", false, "validate descriptors and ring buffers, then exit")
	dump := flag.Bool("dump-descriptors", false, "print USB descriptors, then exit")
	showVer := flag.Bool("version", false, "print version, then exit")
	attachBroker := flag.Bool("attach-broker", false, "run the elevated cable-device broker")
	brokerAddress := flag.String("broker-address", "", "internal broker callback address")
	brokerToken := flag.String("broker-token", "", "internal broker authentication token")
	flag.Parse()

	if *showVer {
		fmt.Printf("Virtual Surround Cables %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}
	if *cables < 1 || *cables > 120 {
		logger.Printf("Invalid cable count %d; falling back to 1", *cables)
		*cables = 1
	}
	if *latencyMS < 20 || *latencyMS > 5000 {
		logger.Printf("Invalid buffer size %d ms; falling back to 250 ms", *latencyMS)
		*latencyMS = 250
	}
	if *attachBroker {
		if err := runAttachBroker(*brokerAddress, *brokerToken, logger); err != nil {
			logger.Printf("Administrator broker failed: %v", err)
			os.Exit(1)
		}
		return
	}

	devices, err := makeDevices(*cables, *latencyMS)
	if err != nil {
		failAndExit(logger, logPath, "Could not create the virtual cable devices", err, *selfTest || *dump)
	}
	if *selfTest {
		if err := runSelfTest(devices); err != nil {
			failAndExit(logger, logPath, "Self-test failed", err, true)
		}
		fmt.Printf("Virtual Surround Cables %s self-test passed.\n", version)
		logger.Printf("Self-test passed")
		return
	}
	if *dump {
		for _, dev := range devices {
			fmt.Printf("\n[%s | bus ID %s]\n", dev.Product(), dev.BusID)
			fmt.Print(hex.Dump(dev.Descriptors.Device))
			fmt.Print(hex.Dump(dev.Descriptors.Config))
		}
		return
	}

	manager := &appServer{
		address:  *address,
		bufferMS: *latencyMS,
		logger:   logger,
		logPath:  logPath,
		count:    *cables,
	}
	if err := manager.Restart(*cables); err != nil {
		// Do not close the GUI. The window remains available so the user can
		// install the driver, inspect the status and open the diagnostic log.
		logger.Printf("Initial USB/IP server start failed; keeping GUI open: %v", err)
	}
	defer manager.Stop()
	runGUI(manager)
	logger.Printf("Virtual Surround Cables window closed normally")
}

func failAndExit(logger *log.Logger, logPath, title string, err error, consoleOnly bool) {
	message := title
	if err != nil {
		message += ": " + err.Error()
	}
	logger.Printf("FATAL: %s", message)
	fmt.Fprintln(os.Stderr, "[ERROR] "+message)
	if !consoleOnly {
		showFatalDialog(message + "\n\nDiagnostic log:\n" + logPath)
	}
	os.Exit(1)
}

func physicalDeviceCount(logicalCables int) int {
	return (logicalCables + surround.MaxCablesPerUSBDevice - 1) / surround.MaxCablesPerUSBDevice
}

func makeDevices(count, latency int) ([]*surround.Device, error) {
	devices := make([]*surround.Device, 0, physicalDeviceCount(count))
	for deviceNumber, firstCable := 1, 1; firstCable <= count; deviceNumber, firstCable = deviceNumber+1, firstCable+surround.MaxCablesPerUSBDevice {
		cableCount := count - firstCable + 1
		if cableCount > surround.MaxCablesPerUSBDevice {
			cableCount = surround.MaxCablesPerUSBDevice
		}
		d, err := surround.NewDevice(deviceNumber, firstCable, cableCount, latency)
		if err != nil {
			return nil, fmt.Errorf("create USB device %d: %w", deviceNumber, err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func runSelfTest(devices []*surround.Device) error {
	for _, dev := range devices {
		if err := dev.Descriptors.Validate(); err != nil {
			return fmt.Errorf("%s: %w", dev.Product(), err)
		}
		if _, status := dev.HandleControl(surround.SetupPacket{RequestType: 0x00, Request: surround.RequestSetConfiguration, Value: 1}, nil); status != 0 {
			return fmt.Errorf("set configuration status %d", status)
		}
		for _, cable := range dev.Cables {
			for _, iface := range []uint8{cable.PlaybackInterface, cable.CaptureInterface} {
				if _, status := dev.HandleControl(surround.SetupPacket{RequestType: 0x01, Request: surround.RequestSetInterface, Index: uint16(iface), Value: 1}, nil); status != 0 {
					return fmt.Errorf("activate interface %d status %d", iface, status)
				}
			}
			pcm := make([]byte, surround.BytesPerAudioFrame)
			for i := range pcm {
				pcm[i] = byte(cable.Number + i)
			}
			if dev.WritePlayback(cable.Endpoint, pcm) != len(pcm) {
				return fmt.Errorf("cable %d PCM write failed", cable.Number)
			}
			out := make([]byte, len(pcm))
			dev.ReadCapture(cable.Endpoint, out)
			for i := range pcm {
				if out[i] != pcm[i] {
					return fmt.Errorf("cable %d PCM loopback mismatch", cable.Number)
				}
			}
		}
	}
	return nil
}

func applicationDataDir() string {
	if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
		return filepath.Join(local, "Virtual Surround Cables")
	}
	base, err := os.UserCacheDir()
	if err == nil && strings.TrimSpace(base) != "" {
		return filepath.Join(base, "Virtual Surround Cables")
	}
	return filepath.Join(os.TempDir(), "Virtual Surround Cables")
}

func configFilePath() string {
	return filepath.Join(applicationDataDir(), "CONFIG.ini")
}

func loadCableCount() int {
	paths := []string{configFilePath(), "CONFIG.ini"}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "CABLES") {
				n, parseErr := strconv.Atoi(strings.TrimSpace(parts[1]))
				if parseErr == nil && n >= 1 && n <= 120 {
					return n
				}
			}
		}
	}
	return 1
}

func saveCableCount(n int) error {
	dir := applicationDataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf("# Virtual Surround Cables configuration\r\nCABLES=%d\r\nCABLES_PER_DEVICE=4\r\nCHANNELS=8\r\nSAMPLE_RATE=48000\r\nBITS=16\r\nLISTEN=127.0.0.3:3240\r\nBUFFER_MS=250\r\n", n)
	return os.WriteFile(configFilePath(), []byte(content), 0o644)
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	if clean, err := filepath.Abs(path); err == nil {
		return clean
	}
	return path
}

func newApplicationLogger() (*log.Logger, *os.File, string) {
	candidateDirs := []string{
		applicationDataDir(),
		filepath.Join(filepath.Dir(executablePath()), "Logs"),
		filepath.Join(os.TempDir(), "Virtual Surround Cables"),
	}

	for _, dir := range candidateDirs {
		if strings.TrimSpace(dir) == "" || dir == "." {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		path := filepath.Join(dir, "VirtualSurroundCables_USBIP.log")
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			continue
		}
		// A Windows GUI process often has no valid stdout handle (for example
		// when started hidden or from Explorer). Put the persistent file first:
		// MultiWriter stops at the first write error, so stdout-first silently
		// prevented every runtime log entry from reaching disk in that case.
		writer := io.MultiWriter(file, os.Stdout)
		return log.New(writer, "", log.Ldate|log.Ltime|log.Lmicroseconds), file, path
	}

	path := filepath.Join(os.TempDir(), "VirtualSurroundCables_USBIP.log")
	return log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds), nil, path
}
