package surround

import (
	"encoding/binary"
	"testing"
)

func TestMultiCableDescriptorsValidate(t *testing.T) {
	d, err := NewDescriptors(1, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := int(binary.LittleEndian.Uint16(d.Config[2:4])); got != len(d.Config) {
		t.Fatalf("total length=%d actual=%d", got, len(d.Config))
	}
	if got := binary.LittleEndian.Uint16(d.Device[2:4]); got != 0x0200 {
		t.Fatalf("bcdUSB=0x%04x", got)
	}
	if d.InterfaceCount() != 9 {
		t.Fatalf("interfaces=%d want 9", d.InterfaceCount())
	}
	const acHeaderOffset = 18 // configuration header + standard AC interface
	acTotal := int(binary.LittleEndian.Uint16(d.Config[acHeaderOffset+5 : acHeaderOffset+7]))
	firstStreaming := -1
	for pos := 0; pos < len(d.Config); {
		length := int(d.Config[pos])
		if d.Config[pos+1] == DescriptorInterface && d.Config[pos+2] == 1 && d.Config[pos+3] == 0 {
			firstStreaming = pos
			break
		}
		pos += length
	}
	if firstStreaming < 0 || acTotal != firstStreaming-acHeaderOffset {
		t.Fatalf("AC total length=%d, class-specific bytes=%d", acTotal, firstStreaming-acHeaderOffset)
	}
}

func TestFourCablesHaveIndependentHighSpeedEndpoints(t *testing.T) {
	d, _ := NewDescriptors(1, 1, 4)
	var endpoints [][]byte
	for pos := 0; pos < len(d.Config); {
		length := int(d.Config[pos])
		if d.Config[pos+1] == DescriptorEndpoint {
			endpoints = append(endpoints, d.Config[pos:pos+length])
		}
		pos += length
	}
	if len(endpoints) != 8 {
		t.Fatalf("endpoint count=%d want 8", len(endpoints))
	}
	for cable := 0; cable < 4; cable++ {
		out, in := endpoints[cable*2], endpoints[cable*2+1]
		if out[2] != byte(cable+1) || in[2] != byte(0x81+cable) {
			t.Fatalf("cable %d endpoints OUT=%02x IN=%02x", cable+1, out[2], in[2])
		}
		for _, ep := range [][]byte{out, in} {
			if got := binary.LittleEndian.Uint16(ep[4:6]); got != BytesPerMillisecond {
				t.Fatalf("packet=%d want %d", got, BytesPerMillisecond)
			}
			if ep[6] != 4 {
				t.Fatalf("high-speed interval=%d want 4", ep[6])
			}
		}
	}
}

func TestUniqueProductID(t *testing.T) {
	a, _ := NewDescriptors(1, 1, 4)
	b, _ := NewDescriptors(2, 5, 4)
	if binary.LittleEndian.Uint16(a.Device[10:12]) == binary.LittleEndian.Uint16(b.Device[10:12]) {
		t.Fatal("product IDs must differ")
	}
}

func TestDescriptorLimits(t *testing.T) {
	if _, err := NewDescriptors(1, 1, 5); err == nil {
		t.Fatal("expected per-device limit error")
	}
	if _, err := NewDescriptors(30, 117, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDescriptors(30, 118, 4); err == nil {
		t.Fatal("expected global cable limit error")
	}
}
