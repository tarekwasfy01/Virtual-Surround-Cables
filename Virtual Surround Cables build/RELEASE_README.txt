VIRTUAL SURROUND CABLES 1.1.0
================================

The required signed usbip-win2 driver files are included in this release.
On first start, approve the single administrator request. The app installs the
driver directly without a separate setup program and then creates the devices.

Virtual Surround Cables creates independent 7.1 audio loopback cables.
One virtual USB device contains up to four cables, allowing up to 120 logical
cables with the 30-port usbip-win2 hub.

Audio format: 8 channels, 48 kHz, 16-bit PCM.
Default: 8 cables across 2 USB devices.
Local USB/IP server: 127.0.0.3:3240.

Start Virtual Surround Cables.exe and approve the administrator request once.
Use Add Cable and Remove Cable to change the logical endpoint count.

Windows may require one restart after the first driver installation.

This is experimental software. Test it before production or live use.
