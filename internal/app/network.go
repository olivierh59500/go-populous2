package app

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"go-populous2/internal/engine"
	"go-populous2/internal/network"
)

type NetworkStatus struct {
	Message     string
	Ready, Busy bool
	Side        int
	Failure     error
}
type networkUpdate struct {
	world   *engine.World
	session *network.Session
	results []network.CommandResult
	err     error
}

// NetworkController keeps all mutable simulation work off the Ebitengine
// thread. A worker owns a detached World; Poll publishes only finished rounds.
// The framebuffer always reads the previous stable world while TCP waits.
type NetworkController struct {
	mu                     sync.Mutex
	listen, connect, rules string
	ctx                    context.Context
	cancel                 context.CancelFunc
	listener               net.Listener
	connection             net.Conn
	session                *network.Session
	updates                chan networkUpdate
	queued                 []network.Command
	status                 NetworkStatus
	started, closed        bool
}

func NewNetworkController(listen, connect, rules string) (*NetworkController, error) {
	if (listen == "") == (connect == "") || rules == "" {
		return nil, fmt.Errorf("choose one host or join address")
	}
	for _, address := range []string{listen, connect} {
		if address != "" {
			if _, _, err := net.SplitHostPort(address); err != nil {
				return nil, fmt.Errorf("network address: %w", err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &NetworkController{listen: listen, connect: connect, rules: rules, ctx: ctx, cancel: cancel, updates: make(chan networkUpdate, 1), status: NetworkStatus{Side: -1}}, nil
}

func (c *NetworkController) Start(world *engine.World) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("network connection is closed")
	}
	if c.started {
		return nil
	}
	var candidate *engine.World
	if c.listen != "" {
		if world == nil {
			return fmt.Errorf("select a world before hosting")
		}
		var err error
		candidate, err = world.Snapshot().Restore()
		if err != nil {
			return err
		}
		for owner := range candidate.Players {
			candidate.Players[owner].Computer = false
		}
		c.status.Message = "Waiting for the other player"
	} else {
		c.status.Message = "Connecting to the other player"
	}
	c.started, c.status.Busy = true, true
	go c.handshake(candidate)
	return nil
}
func (c *NetworkController) handshake(world *engine.World) {
	var connection net.Conn
	var err error
	if c.listen != "" {
		var listener net.Listener
		listener, err = net.Listen("tcp", c.listen)
		if err == nil {
			c.mu.Lock()
			c.listener = listener
			closed := c.closed
			c.mu.Unlock()
			if closed {
				listener.Close()
				return
			}
			connection, err = listener.Accept()
			listener.Close()
		}
	} else {
		dialContext, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		defer cancel()
		for {
			connection, err = (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(dialContext, "tcp", c.connect)
			if err == nil {
				break
			}
			select {
			case <-dialContext.Done():
				err = dialContext.Err()
			case <-time.After(100 * time.Millisecond):
				continue
			}
			break
		}
	}
	if err != nil {
		c.publish(networkUpdate{err: err})
		return
	}
	c.mu.Lock()
	c.connection = connection
	closed := c.closed
	c.mu.Unlock()
	if closed {
		connection.Close()
		return
	}
	ctx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	var session *network.Session
	if c.listen != "" {
		session, err = network.Host(ctx, connection, world, c.rules)
	} else {
		session, world, err = network.Join(ctx, connection, c.rules)
	}
	if err != nil {
		connection.Close()
		c.publish(networkUpdate{err: err})
		return
	}
	c.publish(networkUpdate{session: session, world: world})
}
func (c *NetworkController) publish(update networkUpdate) {
	select {
	case c.updates <- update:
	case <-c.ctx.Done():
		if update.session != nil {
			update.session.Close()
		}
	}
}

func (c *NetworkController) Submit(command network.Command) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.status.Failure != nil {
		return fmt.Errorf("multiplayer is paused")
	}
	if !c.status.Ready {
		return fmt.Errorf("waiting for the other player")
	}
	if len(c.queued) >= 64 {
		return fmt.Errorf("input queue is full")
	}
	c.queued = append(c.queued, command)
	return nil
}

// BeginRound starts at most one exchange. Local commands remain queued while a
// previous round waits, so a slow connection cannot discard player input.
func (c *NetworkController) BeginRound(world *engine.World) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || !c.status.Ready || c.status.Busy || c.status.Failure != nil {
		return false
	}
	if world == nil {
		c.status.Failure = fmt.Errorf("multiplayer world is missing")
		return false
	}
	candidate, err := world.Snapshot().Restore()
	if err != nil {
		c.status.Failure = err
		return false
	}
	commands := append([]network.Command(nil), c.queued...)
	c.queued = nil
	session := c.session
	c.status.Busy = true
	go func() {
		ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		defer cancel()
		results, err := session.Advance(ctx, candidate, commands)
		c.publish(networkUpdate{world: candidate, results: results, err: err})
	}()
	return true
}

func (c *NetworkController) Poll() (*engine.World, []network.CommandResult, NetworkStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case update := <-c.updates:
		c.status.Busy = false
		if update.err != nil {
			c.status.Failure = update.err
			c.status.Message = "Connection interrupted; game paused"
			return nil, nil, c.status
		}
		if update.session != nil {
			c.session = update.session
			c.status.Ready = true
			c.status.Side = update.session.Side()
			c.status.Message = "Two-player game connected"
		}
		return update.world, update.results, c.status
	default:
		return nil, nil, c.status
	}
}
func (c *NetworkController) Status() NetworkStatus { c.mu.Lock(); defer c.mu.Unlock(); return c.status }
func (c *NetworkController) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.cancel()
	listener, connection, session := c.listener, c.connection, c.session
	c.mu.Unlock()
	if listener != nil {
		listener.Close()
	}
	if session != nil {
		session.Close()
	} else if connection != nil {
		connection.Close()
	}
}

// ConfigureNetwork selects an explicit desktop host/join connection. Hosting
// starts after the normal world selection; joining waits for the host's world.
func (g *Game) ConfigureNetwork(listen, connect string) error {
	if listen == "" && connect == "" {
		return nil
	}
	controller, err := NewNetworkController(listen, connect, g.Assets.RulesID)
	if err != nil {
		return err
	}
	if g.Network != nil {
		g.Network.Close()
	}
	g.Network = controller
	if connect != "" {
		return controller.Start(nil)
	}
	return nil
}

func (g *Game) pollNetwork() error {
	if g.Network == nil {
		return nil
	}
	world, results, status := g.Network.Poll()
	if world != nil {
		first := g.World == nil || g.Screen != Playing
		g.World, g.Screen = world, Playing
		if first {
			leader := world.Players[status.Side].Leader
			if leader > 0 {
				f := world.Followers[leader]
				g.CameraX = max(0, min(56, int(f.X)-3))
				g.CameraY = max(0, min(56, int(f.Y)-3))
			}
		}
	}
	for _, result := range results {
		if result.Side == status.Side && result.Error != "" {
			g.Message, g.messageUntil = result.Error, g.Updates+100
		}
	}
	if status.Failure != nil {
		g.Message, g.messageUntil = status.Message, g.Updates+150
	} else if !status.Ready && status.Message != "" {
		g.Message, g.messageUntil = status.Message, g.Updates+50
	}
	return nil
}
func (g *Game) advanceNetwork() {
	if g.Network != nil && g.World != nil && g.Updates%4 == 0 {
		g.Network.BeginRound(g.World)
	}
}
func (g *Game) submitNetwork(command network.Command) error {
	if g.Network == nil {
		return fmt.Errorf("network connection is missing")
	}
	return g.Network.Submit(command)
}

func (g *Game) playerSide() int {
	if g.Network != nil {
		side := g.Network.Status().Side
		if side >= 0 && side < 2 {
			return side
		}
	}
	return 0
}
