package surround

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

const (
	DescriptorDevice          = 0x01
	DescriptorConfiguration   = 0x02
	DescriptorString          = 0x03
	DescriptorInterface       = 0x04
	DescriptorEndpoint        = 0x05
	DescriptorDeviceQualifier = 0x06
	DescriptorCSInterface     = 0x24
	DescriptorCSEndpoint      = 0x25

	Channels              = 8
	SampleRate            = 48000
	BitsPerSample         = 16
	BytesPerAudioFrame    = Channels * (BitsPerSample / 8)
	BytesPerMillisecond   = SampleRate * BytesPerAudioFrame / 1000
	MaxCablesPerUSBDevice = 4
)

// Descriptors describes one high-speed USB Audio Class 1.0 device containing
// several independent 7.1 playback/capture loopback cables.
type Descriptors struct {
	DeviceNumber    int
	FirstCable      int
	CableCount      int
	Product         string
	Serial          string
	Device          []byte
	DeviceQualifier []byte
	Config          []byte
	Strings         map[uint8][]byte
}

func NewDescriptors(deviceNumber, firstCable, cableCount int) (*Descriptors, error) {
	if deviceNumber < 1 || deviceNumber > 30 {
		return nil, fmt.Errorf("device number must be 1..30")
	}
	if firstCable < 1 || firstCable > 120 {
		return nil, fmt.Errorf("first cable must be 1..120")
	}
	if cableCount < 1 || cableCount > MaxCablesPerUSBDevice || firstCable+cableCount-1 > 120 {
		return nil, fmt.Errorf("cable count must fit 1..120 and be at most %d per device", MaxCablesPerUSBDevice)
	}
	lastCable := firstCable + cableCount - 1
	product := fmt.Sprintf("Virtual Surround Device %02d (Cables %03d-%03d)", deviceNumber, firstCable, lastCable)
	serial := fmt.Sprintf("VSURROUND-R2-%02d-%03d", deviceNumber, firstCable)
	d := &Descriptors{
		DeviceNumber: deviceNumber,
		FirstCable:   firstCable,
		CableCount:   cableCount,
		Product:      product,
		Serial:       serial,
		Strings:      make(map[uint8][]byte),
	}
	d.Device = deviceDescriptor(deviceNumber)
	d.DeviceQualifier = deviceQualifierDescriptor()
	d.Config = configurationDescriptor(firstCable, cableCount)
	d.Strings[0] = []byte{4, DescriptorString, 0x09, 0x04}
	d.Strings[1] = stringDescriptor("Tarek Wasfy and AI")
	d.Strings[2] = stringDescriptor(product)
	d.Strings[3] = stringDescriptor(serial)
	for local := 0; local < cableCount; local++ {
		number := firstCable + local
		d.Strings[uint8(4+local*2)] = stringDescriptor(fmt.Sprintf("Surround Cable %03d Playback (7.1)", number))
		d.Strings[uint8(5+local*2)] = stringDescriptor(fmt.Sprintf("Surround Cable %03d Recording (7.1)", number))
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}

func deviceDescriptor(deviceNumber int) []byte {
	pid := uint16(0xCD00 + deviceNumber)
	b := make([]byte, 18)
	b[0], b[1] = 18, DescriptorDevice
	binary.LittleEndian.PutUint16(b[2:4], 0x0200)
	b[7] = 64
	binary.LittleEndian.PutUint16(b[8:10], 0xFFFF)
	binary.LittleEndian.PutUint16(b[10:12], pid)
	binary.LittleEndian.PutUint16(b[12:14], 0x0001)
	b[14], b[15], b[16], b[17] = 1, 2, 3, 1
	return b
}

func deviceQualifierDescriptor() []byte {
	return []byte{10, DescriptorDeviceQualifier, 0x00, 0x02, 0, 0, 0, 64, 1, 0}
}

func configurationDescriptor(firstCable, cableCount int) []byte {
	var b []byte
	add := func(x ...byte) { b = append(b, x...) }
	interfaceCount := 1 + cableCount*2
	add(9, DescriptorConfiguration, 0, 0, byte(interfaceCount), 1, 0, 0x80, 50)

	// AudioControl interface and its class-specific header.
	add(9, DescriptorInterface, 0, 0, 0, 0x01, 0x01, 0x00, 0)
	headerLength := 8 + cableCount*2
	// Every 7.1 cable contributes two 12-byte terminals, two 16-byte feature
	// units and two 9-byte terminals: 74 class-specific AC bytes in total.
	acTotalLength := headerLength + cableCount*74
	add(byte(headerLength), DescriptorCSInterface, 0x01, 0x00, 0x01, byte(acTotalLength), byte(acTotalLength>>8), byte(cableCount*2))
	for local := 0; local < cableCount; local++ {
		add(byte(1+local*2), byte(2+local*2))
	}

	// Each cable owns six topology entities.
	for local := 0; local < cableCount; local++ {
		entity := byte(1 + local*6)
		playbackName := byte(4 + local*2)
		captureName := playbackName + 1
		// USB streaming input -> feature unit -> 7.1 speaker output.
		add(12, DescriptorCSInterface, 0x02, entity, 0x01, 0x01, 0, Channels, 0x3F, 0x06, 0, 0)
		appendFeatureUnit(&b, entity+1, entity)
		add(9, DescriptorCSInterface, 0x03, entity+2, 0x01, 0x03, 0, entity+1, playbackName)
		// 7.1 microphone input -> feature unit -> USB streaming output.
		add(12, DescriptorCSInterface, 0x02, entity+3, 0x01, 0x02, 0, Channels, 0x3F, 0x06, 0, captureName)
		appendFeatureUnit(&b, entity+4, entity+3)
		add(9, DescriptorCSInterface, 0x03, entity+5, 0x01, 0x01, 0, entity+4, 0)
	}

	for local := 0; local < cableCount; local++ {
		entity := byte(1 + local*6)
		playbackInterface := byte(1 + local*2)
		captureInterface := playbackInterface + 1
		endpoint := byte(1 + local)

		add(9, DescriptorInterface, playbackInterface, 0, 0, 0x01, 0x02, 0x00, 0)
		add(9, DescriptorInterface, playbackInterface, 1, 1, 0x01, 0x02, 0x00, 0)
		add(7, DescriptorCSInterface, 0x01, entity, 1, 0x01, 0x00)
		add(11, DescriptorCSInterface, 0x02, 1, Channels, 2, BitsPerSample, 1, 0x80, 0xBB, 0x00)
		add(9, DescriptorEndpoint, endpoint, 0x09, 0x00, 0x03, 4, 0, 0)
		add(7, DescriptorCSEndpoint, 0x01, 0, 0, 0, 0)

		add(9, DescriptorInterface, captureInterface, 0, 0, 0x01, 0x02, 0x00, 0)
		add(9, DescriptorInterface, captureInterface, 1, 1, 0x01, 0x02, 0x00, 0)
		add(7, DescriptorCSInterface, 0x01, entity+5, 1, 0x01, 0x00)
		add(11, DescriptorCSInterface, 0x02, 1, Channels, 2, BitsPerSample, 1, 0x80, 0xBB, 0x00)
		add(9, DescriptorEndpoint, 0x80|endpoint, 0x0D, 0x00, 0x03, 4, 0, 0)
		add(7, DescriptorCSEndpoint, 0x01, 0, 0, 0, 0)
	}

	binary.LittleEndian.PutUint16(b[2:4], uint16(len(b)))
	return b
}

func appendFeatureUnit(b *[]byte, unitID, sourceID byte) {
	controls := make([]byte, Channels+1)
	controls[0] = 0x03 // master mute and volume
	*b = append(*b, byte(7+len(controls)), DescriptorCSInterface, 0x06, unitID, sourceID, 1)
	*b = append(*b, controls...)
	*b = append(*b, 0)
}

func stringDescriptor(s string) []byte {
	encoded := utf16.Encode([]rune(s))
	if len(encoded) > 126 {
		encoded = encoded[:126]
	}
	b := make([]byte, 2+len(encoded)*2)
	b[0], b[1] = byte(len(b)), DescriptorString
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(b[2+i*2:], v)
	}
	return b
}

func (d *Descriptors) GetDescriptor(descriptorType, index uint8) ([]byte, bool) {
	var v []byte
	var ok bool
	switch descriptorType {
	case DescriptorDevice:
		if index == 0 {
			v, ok = d.Device, true
		}
	case DescriptorConfiguration:
		if index == 0 {
			v, ok = d.Config, true
		}
	case DescriptorDeviceQualifier:
		if index == 0 {
			v, ok = d.DeviceQualifier, true
		}
	case DescriptorString:
		v, ok = d.Strings[index]
	}
	return append([]byte(nil), v...), ok
}

func (d *Descriptors) InterfaceCount() int { return 1 + d.CableCount*2 }

func (d *Descriptors) Validate() error {
	if len(d.Device) != 18 || d.Device[1] != DescriptorDevice {
		return fmt.Errorf("invalid device descriptor")
	}
	if len(d.DeviceQualifier) != 10 || d.DeviceQualifier[1] != DescriptorDeviceQualifier {
		return fmt.Errorf("invalid device qualifier")
	}
	if len(d.Config) < 9 || d.Config[1] != DescriptorConfiguration {
		return fmt.Errorf("invalid configuration descriptor")
	}
	if int(binary.LittleEndian.Uint16(d.Config[2:4])) != len(d.Config) {
		return fmt.Errorf("invalid configuration total length")
	}
	if int(d.Config[4]) != d.InterfaceCount() {
		return fmt.Errorf("configuration interface count mismatch")
	}
	for pos := 0; pos < len(d.Config); {
		length := int(d.Config[pos])
		if length < 2 || pos+length > len(d.Config) {
			return fmt.Errorf("invalid descriptor at offset %d", pos)
		}
		pos += length
	}
	return nil
}
