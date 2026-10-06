package game

import (
	"context"
	"fmt"
	"time"

	"go-populous2/internal/populous2"
)

// SetNetwork configures a real stream endpoint. The serial requester still
// owns native profile/baud/connect choices; TCP never invents a successful
// handshake or repairs divergent game state.
func (g *NativeGame) SetNetwork(listen, connect string) error {
	if listen != "" && connect != "" {
		return fmt.Errorf("choose either listen or connect")
	}
	var err error
	if listen != "" {
		g.Network, err = populous2.ListenNativeNetwork(listen)
	} else if connect != "" {
		g.Network, err = populous2.DialNativeNetwork(context.Background(), connect)
	}
	return err
}

func (g *NativeGame) networkReady() error {
	if g.Network == nil || g.Transport != nil {
		return nil
	}
	connection, ready, err := g.Network.Poll()
	if err != nil {
		return err
	}
	if !ready {
		return nil
	}
	conn, err := populous2.NewNativeSerialConn(connection)
	if err != nil {
		return err
	}
	waitSite := uint32(0)
	var waitStart time.Time
	transport, err := g.Host.NewTransport(conn, populous2.NativeTransportFrameCallbacks{WaitCPU: func(site, iterations uint32) (bool, error) {
		if iterations != 100000 {
			return false, fmt.Errorf("native network wait count unavailable")
		}
		if waitSite != site {
			waitSite, waitStart = site, time.Now()
		}
		// Portable configured elapsed time for the original CPU spin loop.
		// This is not an instruction-cycle or serial-baud emulation claim.
		return time.Since(waitStart) >= 250*time.Millisecond, nil
	}, CallTransport: g.networkChild, NativeFileFrameCallbacks: populous2.NativeFileFrameCallbacks{Sound: func(cue uint16, c *populous2.NativeFrameRegisterContext) error { return g.Operations.DirectCue(cue, c) }}}, func(bool, *populous2.NativeFrameRegisterContext) error { return nil })
	if err != nil {
		conn.Close()
		return err
	}
	g.Transport = transport
	return nil
}

func (g *NativeGame) networkChild(call populous2.NativeFileFrameCall, phase *uint32) (populous2.NativeSerialFrameChildResult, error) {
	if call.Routine != 0x10ad8 {
		return populous2.NativeSerialFrameChildResult{}, fmt.Errorf("native network source child%x unavailable", call.Routine)
	}
	if g.NetworkStartup == nil {
		var err error
		g.NetworkStartup, err = populous2.NewNativeRuntimeResetDirector(0x10ad8)
		if err != nil {
			return populous2.NativeSerialFrameChildResult{}, err
		}
	}
	step, err := g.NetworkStartup.Advance(g.Host, &g.Rules, call.Frame, g.Startup)
	if step.Complete {
		g.NetworkStartup = nil
		g.networkRefresh = true
	}
	return populous2.NativeSerialFrameChildResult{Complete: step.Complete, Zero: step.Zero, Negative: step.Negative}, err
}

func (g *NativeGame) serialChild(call populous2.NativeSerialFrameCall, phase *uint32) (populous2.NativeSerialFrameChildResult, error) {
	if err := g.networkReady(); err != nil {
		return populous2.NativeSerialFrameChildResult{}, err
	}
	if g.Transport == nil {
		if g.Network == nil {
			return populous2.NativeSerialFrameChildResult{}, fmt.Errorf("native serial connection requires -listen or -connect")
		}
		return populous2.NativeSerialFrameChildResult{}, nil
	}
	return g.Transport.SerialChild(call, phase)
}

func (g *NativeGame) menuChild(call populous2.NativeFileFrameCall, phase *uint32) (populous2.NativeCommandFrameResult, error) {
	if call.Routine != 0x181c0 || call.Frame == nil || phase == nil {
		return populous2.NativeCommandFrameResult{}, fmt.Errorf("native menu source child%x unavailable", call.Routine)
	}
	if err := g.networkReady(); err != nil {
		return populous2.NativeCommandFrameResult{}, err
	}
	if g.Transport != nil {
		return g.Transport.ResumeMenuChild(call, phase)
	}
	// The original body returns immediately for local game modes; running
	// that body also preserves its actual D0 and condition flags.
	if *phase == 0 {
		g.LocalResume = populous2.NativeTransportResumeState{}
		*phase = 1
	}
	step, err := g.LocalResume.Advance(populous2.NativeTransportFrameCallbacks{NativeFileFrameCallbacks: populous2.NativeFileFrameCallbacks{Frame: call.Frame, Memory: g.Host.Memory.BSS, Code: g.Host.Memory.Code, CodeBase: g.Host.Memory.CodeBase}})
	if err != nil {
		return populous2.NativeCommandFrameResult{}, err
	}
	return populous2.NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, nil
}
