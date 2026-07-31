# Virtual Surround Cables

Virtual Surround Cables is an experimental native Windows application that creates independent 7.1 audio loopback cables as standards-based USB Audio devices over a local USB/IP connection.

## Driver requirement

The **usbip-win2 driver must be installed first**. The application includes the same **Open Downloads** and **Download and Install Driver** workflow as Virtual MIDI Cables. Connecting or changing Windows devices requires one administrator approval per application session.

## What is different

One exported USB device contains up to four complete surround cables. Each logical cable has its own 7.1 playback endpoint, 7.1 recording endpoint and isolated PCM loopback buffer. This uses the limited USB/IP hub ports much more efficiently:

- 1-4 cables use 1 USB device
- 5-8 cables use 2 USB devices
- 120 cables use 30 USB devices

The GUI manages logical cables. It displays the physical USB device and endpoint used by every cable.

## Audio format

- 7.1 surround (8 channels)
- 48,000 Hz
- 16-bit PCM
- Front Left, Front Right, Center, LFE, Back Left, Back Right, Side Left and Side Right
- High-speed USB Audio Class 1.0 descriptors
- Independent playback-to-recording loopback for every cable

## Coexistence

The local server listens on `127.0.0.3:3240`, so it can coexist with Virtual Cables on `127.0.0.1` and Virtual MIDI Cables on `127.0.0.2`.

## Configuration

The default `CONFIG.ini` creates eight logical cables across two USB devices. The GUI can create between 1 and 120 cables. Runtime settings are saved below `%LOCALAPPDATA%\Virtual Surround Cables`.

## Build

Install Go 1.22 or newer and run:

```text
BUILD_VIRTUAL_SURROUND_CABLES.bat
```

The finished application is written to `Release\Virtual Surround Cables.exe`.

## Validation

The automated suite validates descriptor structure, eight-channel packet sizes, unique endpoints, isolated cable buffers, USB/IP device-list encoding and all 120 logical loopbacks. Enumeration on different Windows and usbip-win2 versions should still be tested before distribution.

## Status

This is experimental software. It uses an experimental local VID/PID range and is not a hardware USB implementation. Do not rely on it for safety-critical, live-broadcast or irreplaceable recording workflows without independent testing.

## License

The application source is licensed under the BSD 2-Clause License. Third-party notices are included in `THIRD_PARTY_NOTICES.txt` and `LICENSES`.
