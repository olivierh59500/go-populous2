package populous2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeSerialRequesterAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/serial_requester_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Baud, Rate                 uint16
			Modem, Incoming, TextHex   string
			IncomingByte               int
			Column, Row, Width, Height int
			Actions                    []struct {
				Action int
				Stop   string
				Rate   uint16
			}
		}
		Bytes []struct {
			Byte byte
			Text string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 160 || len(corpus.Bytes) != 256 {
		t.Fatalf("native serial requester corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeSerialRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range corpus.Cases {
		requester, err := NewNativeSerialRequester(rules, c.Baud, []byte(c.Modem))
		if err != nil {
			t.Fatal(err)
		}
		if c.IncomingByte >= 0 {
			requester.IncomingByte(byte(c.IncomingByte))
		}
		plan, err := requester.Plan()
		want, decodeErr := hex.DecodeString(c.TextHex)
		if err != nil || decodeErr != nil || !bytes.Equal(plan.Text, want) || requester.Baud() != c.Rate || string(requester.Incoming) != c.Incoming || plan.Column != c.Column || plan.Row != c.Row || plan.Width != c.Width || plan.Height != c.Height {
			t.Fatalf("native serial requester differs baud%d modem%q byte%d", c.Baud, c.Modem, c.IncomingByte)
		}
		for _, action := range c.Actions {
			r, err := NewNativeSerialRequester(rules, c.Rate, []byte(c.Modem))
			if err != nil {
				t.Fatal(err)
			}
			step, err := r.Action(action.Action)
			if err != nil {
				t.Fatal(err)
			}
			if r.Baud() != action.Rate || step.EditModem != (action.Stop == "edit") || step.Connect != (action.Stop == "connect") || step.Cancel != (action.Stop == "cancel") {
				t.Fatal("native serial requester action differs")
			}
			if step.Connect && step.Routine != 0x17eec {
				t.Fatal("serial requester claimed connection without original handshake")
			}
		}
	}
	for _, c := range corpus.Bytes {
		r, err := NewNativeSerialRequester(rules, 300, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.IncomingByte(c.Byte)
		if string(r.Incoming) != c.Text {
			t.Fatalf("native signed received byte%x differs", c.Byte)
		}
	}
	r, _ := NewNativeSerialRequester(rules, 300, nil)
	_, _ = r.Action(6)
	step, err := r.FinishModemEdit([]byte("ATDT123"))
	if err != nil || string(step.ModemBytes) != "ATDT123\r\r" {
		t.Fatal("native modem edit did not request its actual carriage returns")
	}
}
