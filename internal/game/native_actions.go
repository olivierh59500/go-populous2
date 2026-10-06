package game

import "fmt"

func (g *NativeGame) nativeActionPosition(action int) (int, int, error) {
	code := g.Host.Memory.Code
	start, err := code.Read16(0xab4e)
	if err != nil {
		return 0, 0, err
	}
	width, err := code.Read16(0xab54)
	if err != nil {
		return 0, 0, err
	}
	x, err := code.Read16(0xab50)
	if err != nil {
		return 0, 0, err
	}
	y, err := code.Read16(0xab52)
	if err != nil {
		return 0, 0, err
	}
	count := 0
	for i := 0; i < 8192; i++ {
		v, err := code.Read8(0xab4e + int(int16(start)) + i)
		if err != nil {
			return 0, 0, err
		}
		if v == 0 {
			break
		}
		if int8(v) > 0x5a {
			flag, err := code.Read8(0x4e92 + int(v-0x5b))
			if err != nil {
				return 0, 0, err
			}
			if int8(flag) > 0 {
				count += 2
				if count == action {
					return (int(x) + i%(int(width)+1)) * 8, int(y) + i/(int(width)+1)*8, nil
				}
			}
		}
	}
	return 0, 0, fmt.Errorf("native requester action%d unavailable", action)
}
