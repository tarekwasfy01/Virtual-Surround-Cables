package surround

import "testing"

func configuredDevice(t *testing.T) *Device {
	t.Helper()
	d, err := NewDevice(1, 1, 4, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, status := d.HandleControl(SetupPacket{RequestType: 0, Request: RequestSetConfiguration, Value: 1}, nil); status != 0 {
		t.Fatalf("configuration status=%d", status)
	}
	return d
}

func TestGetDeviceAndQualifierDescriptors(t *testing.T) {
	d := configuredDevice(t)
	for typ, want := range map[uint8]int{DescriptorDevice: 18, DescriptorDeviceQualifier: 10} {
		data, status := d.HandleControl(SetupPacket{RequestType: 0x80, Request: RequestGetDescriptor, Value: uint16(typ) << 8, Length: 255}, nil)
		if status != 0 || len(data) != want {
			t.Fatalf("type=%d status=%d len=%d", typ, status, len(data))
		}
	}
}

func TestIndependentCableLoopbacks(t *testing.T) {
	d := configuredDevice(t)
	for _, c := range d.Cables {
		for _, iface := range []uint8{c.PlaybackInterface, c.CaptureInterface} {
			if _, status := d.HandleControl(SetupPacket{RequestType: 0x01, Request: RequestSetInterface, Index: uint16(iface), Value: 1}, nil); status != 0 {
				t.Fatalf("interface %d status=%d", iface, status)
			}
		}
		pcm := []byte{byte(c.Number), 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		if d.WritePlayback(c.Endpoint, pcm) != len(pcm) {
			t.Fatalf("cable %d write", c.Number)
		}
		out := make([]byte, len(pcm))
		d.ReadCapture(c.Endpoint, out)
		for i := range pcm {
			if out[i] != pcm[i] {
				t.Fatalf("cable %d byte %d mismatch", c.Number, i)
			}
		}
	}
}

func TestCableBuffersDoNotCross(t *testing.T) {
	d := configuredDevice(t)
	for _, c := range d.Cables {
		for _, iface := range []uint8{c.PlaybackInterface, c.CaptureInterface} {
			d.HandleControl(SetupPacket{RequestType: 1, Request: RequestSetInterface, Index: uint16(iface), Value: 1}, nil)
		}
	}
	d.WritePlayback(1, []byte{1, 1, 1, 1})
	d.WritePlayback(2, []byte{2, 2, 2, 2})
	a, b := make([]byte, 4), make([]byte, 4)
	d.ReadCapture(1, a)
	d.ReadCapture(2, b)
	if a[0] != 1 || b[0] != 2 {
		t.Fatalf("crossed buffers: %v %v", a, b)
	}
}

func TestSamplingRateOnEveryEndpoint(t *testing.T) {
	d := configuredDevice(t)
	for ep := uint8(1); ep <= 4; ep++ {
		for _, addr := range []uint8{ep, 0x80 | ep} {
			data, status := d.HandleControl(SetupPacket{RequestType: 0xA2, Request: AudioGetCur, Value: 0x0100, Index: uint16(addr), Length: 3}, nil)
			if status != 0 || len(data) != 3 || data[0] != 0x80 || data[1] != 0xBB || data[2] != 0 {
				t.Fatalf("endpoint %02x data=%v status=%d", addr, data, status)
			}
		}
	}
}
