package populous2

import (
	"context"
	"fmt"
)

// NativeNetworkSession retains a configured endpoint across explicit source
// serial-requester visits. A retry replaces only the dead host byte stream;
// no native profiles, commands, RNG or handshake results are reconstructed.
type NativeNetworkSession struct {
	Listen, Connect string
	Endpoint        *NativeNetworkEndpoint
	Serial          *NativeSerialConn
	received        [381]byte
	retainReceived  bool
}

func NewNativeNetworkSession(listen, connect string) (*NativeNetworkSession, error) {
	if (listen == "") == (connect == "") {
		return nil, fmt.Errorf("native network requires one listen or connect address")
	}
	s := &NativeNetworkSession{Listen: listen, Connect: connect}
	if err := s.open(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *NativeNetworkSession) open() error {
	var err error
	if s.Listen != "" {
		s.Endpoint, err = ListenNativeNetwork(s.Listen)
		if err == nil {
			s.Listen = s.Endpoint.Address()
		}
	} else {
		s.Endpoint, err = DialNativeNetwork(context.Background(), s.Connect)
	}
	return err
}

func (s *NativeNetworkSession) Poll() (*NativeSerialConn, bool, error) {
	if s == nil || s.Endpoint == nil {
		return nil, false, fmt.Errorf("native network session missing")
	}
	if s.Serial != nil {
		return s.Serial, true, nil
	}
	connection, ready, err := s.Endpoint.Poll()
	if err != nil || !ready {
		return nil, ready, err
	}
	if s.retainReceived {
		s.Serial, err = newNativeSerialConn(connection, s.received)
		s.retainReceived = false
	} else {
		s.Serial, err = NewNativeSerialConn(connection)
	}
	return s.Serial, err == nil, err
}

// Retry is an explicit source-menu boundary, never called by ordinary frame
// polling. A live stream is retained. The caller verifies that the original
// disconnect/fallback has completed before discarding any native transport.
func (s *NativeNetworkSession) Retry() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("native network retry session missing")
	}
	if s.Endpoint == nil {
		return true, s.open()
	}
	if s.Serial == nil || s.Serial.TerminalError() == nil {
		return false, nil
	}
	s.received, _, _ = s.Serial.NativeTransportReceiveImage()
	s.retainReceived = true
	if err := s.Endpoint.Close(); err != nil {
		return false, err
	}
	_ = s.Serial.Close() // Endpoint already closed the same connection.
	s.Endpoint, s.Serial = nil, nil
	if err := s.open(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *NativeNetworkSession) Close() error {
	if s == nil {
		return nil
	}
	if s.Endpoint != nil {
		return s.Endpoint.Close()
	}
	if s.Serial != nil {
		return s.Serial.Close()
	}
	return nil
}
