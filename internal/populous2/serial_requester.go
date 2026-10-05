package populous2

import "fmt"

type NativeSerialRequester struct {
	Rules           *NativeSerialRules
	BaudIndex       int
	Modem, Incoming []byte
	Editing         bool
	Cancelled       bool
}
type NativeSerialRequesterAction struct {
	EditModem, Connect, Cancel bool
	Routine                    uint32
	ModemBytes                 []byte
}

func NewNativeSerialRequester(r *NativeSerialRules, baud uint16, modem []byte) (*NativeSerialRequester, error) {
	if r == nil {
		return nil, fmt.Errorf("native serial requester rules missing")
	}
	requester := &NativeSerialRequester{Rules: r, Modem: nativeFileCString(modem)}
	for i, value := range r.BaudRates {
		if value == baud {
			requester.BaudIndex = i
			break
		}
	}
	return requester, nil
}
func (r *NativeSerialRequester) Baud() uint16 { return r.Rules.BaudRates[r.BaudIndex] }
func (r *NativeSerialRequester) Plan() (*NativeRequester, error) {
	if r == nil || r.Rules == nil || r.Rules.Presentation == nil {
		return nil, fmt.Errorf("native serial requester missing")
	}
	modem := r.Modem
	if len(modem) == 0 {
		modem = []byte{'k'}
	} // Non-NULL empty native edit buffer.
	return r.Rules.Presentation.Compile(NativeMenuSerial, [][]byte{r.Rules.baudText[r.BaudIndex], modem, r.Incoming})
}

// Action is $4a5a..$4af8. Its five rates are the actual bounded $4b06..$4b2e
// entries; the two following embedded labels are not admitted by this caller.
// Connect requests17eec and does not fabricate a successful connection.
func (r *NativeSerialRequester) Action(action int) (NativeSerialRequesterAction, error) {
	var step NativeSerialRequesterAction
	if r == nil || r.Rules == nil {
		return step, fmt.Errorf("native serial requester missing")
	}
	switch action {
	case 0:
	case 2:
		if r.BaudIndex < 4 {
			r.BaudIndex++
		}
	case 4:
		if r.BaudIndex > 0 {
			r.BaudIndex--
		}
	case 6:
		r.Editing = true
		step.EditModem = true
	case 8:
		step.Connect = true
		step.Routine = 0x17eec
	case 10:
		r.Cancelled = true
		step.Cancel = true
	default:
		return step, fmt.Errorf("native serial requester action%d outside table", action)
	}
	return step, nil
}
func (r *NativeSerialRequester) Click(x, y int) (NativeSerialRequesterAction, error) {
	plan, err := r.Plan()
	if err != nil {
		return NativeSerialRequesterAction{}, err
	}
	return r.Action(r.Rules.Presentation.Requesters.Click(plan, x, y))
}

// FinishModemEdit returns the exact modem bytes requested at $4a7c: a nonempty
// C string followed by two carriage returns. The caller must send them through
// its actual byte transport; the returned action is not a transmission result.
func (r *NativeSerialRequester) FinishModemEdit(value []byte) (NativeSerialRequesterAction, error) {
	var step NativeSerialRequesterAction
	if r == nil || !r.Editing {
		return step, fmt.Errorf("native modem field is not editing")
	}
	r.Modem = nativeFileCString(value)
	r.Editing = false
	if len(r.Modem) != 0 {
		step.ModemBytes = append(append([]byte(nil), r.Modem...), 13, 13)
	}
	return step, nil
}

// IncomingByte is $49de..$4a26. Signed comparisons admit printable bytes below
// 'z'; lowercase a..y becomes uppercase, while z/control/high bytes become a
// space. The original34-byte rolling text is retained without Unicode repair.
func (r *NativeSerialRequester) IncomingByte(value byte) {
	if r == nil {
		return
	}
	if int8(value) < 0x20 || int8(value) >= 0x7a {
		value = 0x20
	}
	if value >= 0x61 {
		value -= 0x20
	}
	if len(r.Incoming) >= 34 {
		r.Incoming = append([]byte(nil), r.Incoming[1:]...)
	}
	r.Incoming = append(r.Incoming, value)
}

func (r *NativeSerialRequester) Poll(port NativeSerialPort) error {
	if r == nil || port.Configure == nil || port.Available == nil || port.Read == nil {
		return fmt.Errorf("native serial requester port missing")
	}
	if err := port.Configure(r.Baud()); err != nil {
		return err
	}
	available, err := port.Available()
	if err != nil {
		return err
	}
	if available == 0 {
		return nil
	}
	var one [1]byte
	n, err := port.Read(one[:])
	if n == 1 {
		r.IncomingByte(one[0])
	}
	if err != nil && err != ErrNativeSerialWait {
		return err
	}
	return nil
}
