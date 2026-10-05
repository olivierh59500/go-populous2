package populous2

import "fmt"

// tickNativeFrameScenario is the complete $17e9a boundary. The event's time
// changes only D0.W before its zero/unsigned-clock tests. A due event executes
// $17500 at the raw $dc4 record with the incoming register continuation.
func (w *World) tickNativeFrameScenario(c *NativeFrameRegisterContext, bindings NativeFrameWorldBindings) (ScenarioScriptStep, error) {
	var result ScenarioScriptStep
	if c == nil {
		return result, fmt.Errorf("native scenario frame context missing")
	}
	memory := w.nativeCleanupMemory()
	offset, e := memory.Read16(0xf0a)
	if e != nil {
		return result, e
	}
	if offset&1 != 0 {
		return result, fmt.Errorf("native scenario frame word read at odd address")
	}
	value, e := memory.Read16(0xdde + int(int16(offset)))
	if e != nil {
		return result, e
	}
	c.Word(0, value)
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	return TickScenarioScript(ScenarioScriptCallbacks{Memory: memory, Execute: func(caller int) error {
		command := c.CommandContext()
		_, e := w.executeNativeNormalCommand(&rules, caller, &command, bindings.Commands)
		c.SetCommandContext(command)
		return e
	}})
}
