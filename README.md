# Virtual Surround Cables

Virtual Surround Cables is a Windows application for creating and managing virtual surround audio cable devices. It enables flexible multichannel audio routing between applications without requiring physical audio cables.

## Features

* Creates virtual surround audio devices
* Routes multichannel audio between Windows applications
* Supports common surround configurations
* Simple graphical user interface
* Automatic device setup
* Start menu and optional desktop shortcuts
* Repair, modify and uninstall support

## System Requirements

* Windows 10 or Windows 11
* Windows build 19041 or newer
* 64-bit Windows installation
* Administrator rights for driver installation

## Installation

1. Download the latest `Virtual-Surround-Cables-Setup.exe` file from the **Releases** section.
2. Run the installer.
3. Follow the installation wizard.
4. Start **Virtual Surround Cables**.
5. Confirm the Windows administrator prompt when the virtual audio driver is installed.

Administrator rights are only required for operations that install, register, update or remove the virtual audio devices.

## Usage

1. Open Virtual Surround Cables.
2. Select the desired surround configuration.
3. Create or activate the virtual cable.
4. Open the Windows sound settings.
5. Select the created virtual device as an input or output device in the required application.

The exact routing options depend on the audio applications and Windows sound configuration.

## Windows Security Notice

Windows SmartScreen may display a warning when running a newly downloaded installer.

Verify that the installer was downloaded from the official GitHub Releases page before continuing.

Driver installation can also be blocked when Windows does not accept the driver signature. In that case, do not disable Windows security features permanently.

## Uninstallation

Virtual Surround Cables can be removed through:

**Settings → Apps → Installed apps → Virtual Surround Cables → Uninstall**

The application may request administrator rights to remove registered virtual audio devices and driver components.

## Troubleshooting

### Virtual device does not appear

* Restart the application as administrator.
* Open Windows sound settings and check disabled devices.
* Restart Windows after installing the driver.
* Verify that the driver installation was not blocked by Windows Security.

### Application cannot create a cable

* Confirm the administrator prompt.
* Close applications currently using the affected audio device.
* Restart the application.
* Use the installer’s repair function.

### No audio is transmitted

* Verify the input and output device selections.
* Check the channel configuration and sample rate.
* Make sure both applications use compatible audio formats.
* Restart applications after changing Windows audio devices.

## Downloads

Official installers and release notes are available on the GitHub **Releases** page.

Recommended download:

```text
Virtual-Surround-Cables-Setup.exe
```

## Reporting Problems

When reporting a problem, include:

* Windows version and build number
* Application version
* Selected surround configuration
* Relevant error message
* Whether the driver installation completed successfully

Do not publish private information or confidential log data.

## License

See the included `LICENSE` file for licensing information.

## Disclaimer

Virtual Surround Cables modifies the Windows audio-device configuration and may install a virtual audio driver. Create a system restore point before installation when testing early or experimental releases.

This project is not affiliated with Microsoft.
