package tgserver

import (
	"net"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/credentials"
)

// shutdownListener tracks accepted sockets, including connections gRPC has
// accepted but has not finished handshaking.
type shutdownListener struct {
	*net.TCPListener

	mu             sync.Mutex
	connectionsMap map[*shutdownConnection]struct{}
}

type shutdownConnection struct {
	*net.TCPConn

	listener *shutdownListener
	detached atomic.Bool
}

// Accept transfers a socket to gRPC unless forced shutdown has started.
func (listener *shutdownListener) Accept() (net.Conn, error) {
	conn, err := listener.AcceptTCP()
	if err != nil {
		return nil, err
	}

	connection := &shutdownConnection{TCPConn: conn, listener: listener}

	listener.mu.Lock()
	if listener.connectionsMap == nil {
		listener.mu.Unlock()
		_ = conn.Close()

		return nil, net.ErrClosed
	}

	listener.connectionsMap[connection] = struct{}{}
	listener.mu.Unlock()

	return connection, nil
}

// forceClose interrupts handshakes as well as active RPCs. gRPC Stop waits for
// pending handshakes before closing its registered transports.
func (listener *shutdownListener) forceClose() {
	_ = listener.Close()

	listener.mu.Lock()
	connectionsMap := listener.connectionsMap
	listener.connectionsMap = nil
	listener.mu.Unlock()

	for connection := range connectionsMap {
		if connection.detached.CompareAndSwap(false, true) {
			_ = connection.TCPConn.Close()
		}
	}
}

// Close releases the connection and removes it from the listener's tracking.
func (connection *shutdownConnection) Close() error {
	connection.detach()

	return connection.TCPConn.Close()
}

// detach relinquishes shutdown ownership while leaving the socket usable.
func (connection *shutdownConnection) detach() {
	connection.detached.Store(true)
	listener := connection.listener
	listener.mu.Lock()
	delete(listener.connectionsMap, connection)
	listener.mu.Unlock()
}

type trackedCredentials struct {
	credentials.TransportCredentials
}

// ServerHandshake removes a dispatched socket from forced-shutdown tracking.
func (tracked trackedCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, authInfo, err := tracked.TransportCredentials.ServerHandshake(rawConn)
	if err == credentials.ErrConnDispatched {
		if connection, ok := rawConn.(*shutdownConnection); ok {
			connection.detach()
		}
	}

	return conn, authInfo, err
}

// Clone preserves dispatch tracking when gRPC clones transport credentials.
func (tracked trackedCredentials) Clone() credentials.TransportCredentials {
	cloned := trackedCredentials{TransportCredentials: tracked.TransportCredentials.Clone()}

	return cloned
}
