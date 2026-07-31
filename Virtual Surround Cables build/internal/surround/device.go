package surround

import (
	"encoding/binary"
	"fmt"
	"sync"

	"virtualsurroundcables/internal/audio"
)

const (
	RequestGetStatus        = 0x00
	RequestClearFeature     = 0x01
	RequestSetFeature       = 0x03
	RequestSetAddress       = 0x05
	RequestGetDescriptor    = 0x06
	RequestSetDescriptor    = 0x07
	RequestGetConfiguration = 0x08
	RequestSetConfiguration = 0x09
	RequestGetInterface     = 0x0A
	RequestSetInterface     = 0x0B
	RequestSynchFrame       = 0x0C
	AudioSetCur             = 0x01
	AudioGetCur             = 0x81
	AudioGetMin             = 0x82
	AudioGetMax             = 0x83
	AudioGetRes             = 0x84
)

type SetupPacket struct {
	RequestType, Request uint8
	Value, Index, Length uint16
}

func ParseSetup(b []byte) (SetupPacket, error) {
	if len(b) < 8 {
		return SetupPacket{}, fmt.Errorf("setup packet too short")
	}
	return SetupPacket{b[0], b[1], binary.LittleEndian.Uint16(b[2:4]), binary.LittleEndian.Uint16(b[4:6]), binary.LittleEndian.Uint16(b[6:8])}, nil
}

type Cable struct {
	Number            int
	PlaybackInterface uint8
	CaptureInterface  uint8
	Endpoint          uint8
	Buffer            *audio.Ring
}

type Device struct {
	Number        int
	BusID         string
	Descriptors   *Descriptors
	Cables        []Cable
	mu            sync.Mutex
	configuration uint8
	altSetting    map[uint8]uint8
	sampleRate    uint32
	mute          map[uint8]bool
	volume        map[uint8]int16
}

func NewDevice(deviceNumber, firstCable, cableCount, latencyMS int) (*Device, error) {
	desc, err := NewDescriptors(deviceNumber, firstCable, cableCount)
	if err != nil {
		return nil, err
	}
	if latencyMS < 10 {
		latencyMS = 10
	}
	if latencyMS > 2000 {
		latencyMS = 2000
	}
	d := &Device{Number: deviceNumber, BusID: fmt.Sprintf("1-%d", deviceNumber), Descriptors: desc, altSetting: map[uint8]uint8{0: 0}, sampleRate: SampleRate, mute: make(map[uint8]bool), volume: make(map[uint8]int16)}
	capacity := SampleRate * BytesPerAudioFrame * latencyMS / 1000
	for local := 0; local < cableCount; local++ {
		playback := uint8(1 + local*2)
		d.Cables = append(d.Cables, Cable{Number: firstCable + local, PlaybackInterface: playback, CaptureInterface: playback + 1, Endpoint: uint8(local + 1), Buffer: audio.NewRing(capacity)})
		d.altSetting[playback], d.altSetting[playback+1] = 0, 0
	}
	return d, nil
}

func (d *Device) Product() string     { return d.Descriptors.Product }
func (d *Device) CableCount() int     { return len(d.Cables) }
func (d *Device) InterfaceCount() int { return d.Descriptors.InterfaceCount() }

func (d *Device) HandleControl(s SetupPacket, out []byte) ([]byte, int32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s.RequestType&0x60 == 0 {
		switch s.Request {
		case RequestGetDescriptor:
			v, ok := d.Descriptors.GetDescriptor(uint8(s.Value>>8), uint8(s.Value))
			if !ok {
				return nil, -32
			}
			return truncate(v, s.Length), 0
		case RequestSetAddress:
			return nil, 0
		case RequestSetConfiguration:
			cfg := uint8(s.Value)
			if cfg > 1 {
				return nil, -32
			}
			d.configuration = cfg
			for k := range d.altSetting {
				d.altSetting[k] = 0
			}
			for i := range d.Cables {
				d.Cables[i].Buffer.Reset()
			}
			return nil, 0
		case RequestGetConfiguration:
			return truncate([]byte{d.configuration}, s.Length), 0
		case RequestSetInterface:
			iface, alt := uint8(s.Index), uint8(s.Value)
			if d.configuration != 1 || alt > 1 {
				return nil, -32
			}
			if _, ok := d.altSetting[iface]; !ok || iface == 0 && alt != 0 {
				return nil, -32
			}
			d.altSetting[iface] = alt
			if alt == 0 {
				if c := d.cableByInterface(iface); c != nil {
					c.Buffer.Reset()
				}
			}
			return nil, 0
		case RequestGetInterface:
			alt, ok := d.altSetting[uint8(s.Index)]
			if !ok {
				return nil, -32
			}
			return truncate([]byte{alt}, s.Length), 0
		case RequestGetStatus:
			return truncate([]byte{0, 0}, s.Length), 0
		case RequestSynchFrame:
			return truncate([]byte{0, 0}, s.Length), 0
		case RequestClearFeature, RequestSetFeature:
			return nil, 0
		default:
			return nil, -32
		}
	}
	recipient := s.RequestType & 0x1f
	selector := uint8(s.Value >> 8)
	endpoint := uint8(s.Index)
	if recipient == 0x02 && selector == 0x01 && d.validEndpoint(endpoint) {
		switch s.Request {
		case AudioSetCur:
			if len(out) >= 3 {
				rate := uint32(out[0]) | uint32(out[1])<<8 | uint32(out[2])<<16
				if rate != SampleRate {
					return nil, -32
				}
				d.sampleRate = rate
			}
			return nil, 0
		case AudioGetCur:
			return truncate(rate24(d.sampleRate), s.Length), 0
		case AudioGetMin, AudioGetMax:
			return truncate(rate24(SampleRate), s.Length), 0
		case AudioGetRes:
			return truncate(rate24(1), s.Length), 0
		}
	}
	entity := uint8(s.Index >> 8)
	iface := uint8(s.Index)
	if recipient == 0x01 && iface == 0 && d.featureEntity(entity) {
		switch uint8(s.Value >> 8) {
		case 0x01:
			switch s.Request {
			case AudioSetCur:
				if len(out) > 0 {
					d.mute[entity] = out[0] != 0
				}
				return nil, 0
			case AudioGetCur:
				if d.mute[entity] {
					return truncate([]byte{1}, s.Length), 0
				}
				return truncate([]byte{0}, s.Length), 0
			}
		case 0x02:
			switch s.Request {
			case AudioSetCur:
				if len(out) >= 2 {
					d.volume[entity] = int16(binary.LittleEndian.Uint16(out[:2]))
				}
				return nil, 0
			case AudioGetCur:
				return truncate(int16LE(d.volume[entity]), s.Length), 0
			case AudioGetMin:
				return truncate(int16LE(-60*256), s.Length), 0
			case AudioGetMax:
				return truncate(int16LE(0), s.Length), 0
			case AudioGetRes:
				return truncate(int16LE(256), s.Length), 0
			}
		}
	}
	return nil, -32
}

func (d *Device) cableByInterface(iface uint8) *Cable {
	for i := range d.Cables {
		if d.Cables[i].PlaybackInterface == iface || d.Cables[i].CaptureInterface == iface {
			return &d.Cables[i]
		}
	}
	return nil
}
func (d *Device) cableByEndpoint(ep uint8) *Cable {
	ep &= 0x7f
	for i := range d.Cables {
		if d.Cables[i].Endpoint == ep {
			return &d.Cables[i]
		}
	}
	return nil
}
func (d *Device) validEndpoint(ep uint8) bool { return d.cableByEndpoint(ep) != nil }
func (d *Device) featureEntity(entity uint8) bool {
	if entity < 2 {
		return false
	}
	r := (entity - 1) % 6
	return r == 1 || r == 4
}

func (d *Device) PlaybackActive(ep uint8) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := d.cableByEndpoint(ep)
	return c != nil && d.configuration == 1 && d.altSetting[c.PlaybackInterface] == 1
}
func (d *Device) CaptureActive(ep uint8) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := d.cableByEndpoint(ep)
	return c != nil && d.configuration == 1 && d.altSetting[c.CaptureInterface] == 1
}
func (d *Device) WritePlayback(ep uint8, p []byte) int {
	if !d.PlaybackActive(ep) {
		return 0
	}
	c := d.cableByEndpoint(ep)
	if c == nil {
		return 0
	}
	return c.Buffer.Write(p)
}
func (d *Device) ReadCapture(ep uint8, p []byte) int {
	if !d.CaptureActive(ep) {
		for i := range p {
			p[i] = 0
		}
		return 0
	}
	c := d.cableByEndpoint(ep)
	if c == nil {
		for i := range p {
			p[i] = 0
		}
		return 0
	}
	return c.Buffer.ReadSilence(p)
}

func truncate(b []byte, n uint16) []byte {
	if int(n) < len(b) {
		b = b[:n]
	}
	return b
}
func rate24(r uint32) []byte { return []byte{byte(r), byte(r >> 8), byte(r >> 16)} }
func int16LE(v int16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}
