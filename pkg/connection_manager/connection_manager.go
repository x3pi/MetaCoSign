package connection_manager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/meta-node-blockchain/meta-node/pkg/connection_manager/connection_client"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pkg_network "github.com/meta-node-blockchain/meta-node/pkg/network"
	"github.com/meta-node-blockchain/meta-node/types/network"
)

// ttlEntry wraps a ConnectionClient with expiration tracking
type ttlEntry struct {
	client    *connection_client.ConnectionClient
	expiresAt time.Time
	ttl       time.Duration
}

// ConnectionManager quản lý các TCP connections với key là string (address hoặc clusterId)
type ConnectionManager struct {
	connections       sync.Map // map[string]network.Connection (for backward compatibility)
	clientConnections sync.Map // map[string]*ConnectionClient (new managed connections)
	ttlConnections    sync.Map // map[string]*ttlEntry (TTL-based connections)
	socketServer      network.SocketServer
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	messageSender     network.MessageSender
	ttlCleanupOnce    sync.Once
}

// NewConnectionManager tạo instance mới của ConnectionManager
func NewConnectionManager(socketServer network.SocketServer, messageSender network.MessageSender) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConnectionManager{
		messageSender: messageSender,
		socketServer:  socketServer,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetMessageSender sets the message sender for creating ConnectionClient instances
func (cm *ConnectionManager) SetMessageSender(messageSender network.MessageSender) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messageSender = messageSender
}

// GetOrCreateConnectionClient returns or creates a ConnectionClient for the given key
func (cm *ConnectionManager) GetOrCreateConnectionClient(key string, connectionAddress string) (*connection_client.ConnectionClient, error) {
	// Check cache first
	logger.Info("GetOrCreateConnectionClient called with key: %s, address: %s", key, connectionAddress)
	if cachedClient, ok := cm.clientConnections.Load(key); ok {
		client := cachedClient.(*connection_client.ConnectionClient)
		if client.IsConnected() {
			return client, nil
		}
		// Connection lost, remove from cache
		cm.clientConnections.Delete(key)
		logger.Warn("Connection lost, removed from cache: %s", key)
	}

	// Create new ConnectionClient
	if cm.messageSender == nil {
		return nil, fmt.Errorf("messageSender not set in ConnectionManager")
	}

	client := connection_client.NewConnectionClient(cm.ctx, key, connectionAddress, cm.messageSender)
	// Connect
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", key, connectionAddress, err)
	}
	// Save to cache
	cm.clientConnections.Store(key, client)
	logger.Info("ConnectionClient created and connected: %s at %s", key, connectionAddress)

	return client, nil
}

// GetOrCreateConnectionClientWithTTL returns or creates a ConnectionClient for the given key,
// with automatic expiration after ttl duration. TTL is reset on each access.
func (cm *ConnectionManager) GetOrCreateConnectionClientWithTTL(key string, connectionAddress string, ttl time.Duration) (*connection_client.ConnectionClient, error) {
	// Start TTL cleanup goroutine on first use
	cm.ttlCleanupOnce.Do(func() {
		go cm.startTTLCleanup()
	})

	// Check cache first
	if cached, ok := cm.ttlConnections.Load(key); ok {
		entry := cached.(*ttlEntry)
		if entry.client.IsConnected() {
			// Reset TTL on access
			entry.expiresAt = time.Now().Add(entry.ttl)
			return entry.client, nil
		}
		// Connection lost, remove from cache
		entry.client.Disconnect()
		cm.ttlConnections.Delete(key)
		logger.Warn("TTL connection lost, removed from cache: %s", key)
	}

	// Create new ConnectionClient
	if cm.messageSender == nil {
		return nil, fmt.Errorf("messageSender not set in ConnectionManager")
	}

	client := connection_client.NewConnectionClient(cm.ctx, key, connectionAddress, cm.messageSender)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", key, connectionAddress, err)
	}

	entry := &ttlEntry{
		client:    client,
		expiresAt: time.Now().Add(ttl),
		ttl:       ttl,
	}
	cm.ttlConnections.Store(key, entry)
	logger.Info("TTL ConnectionClient created: %s at %s (ttl=%s)", key, connectionAddress, ttl)

	return client, nil
}

// startTTLCleanup runs a background goroutine that periodically cleans up expired TTL connections
func (cm *ConnectionManager) startTTLCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			cm.ttlConnections.Range(func(key, value interface{}) bool {
				entry := value.(*ttlEntry)
				if now.After(entry.expiresAt) {
					entry.client.Disconnect()
					cm.ttlConnections.Delete(key)
					logger.Info("TTL connection expired, disconnected: %s", key.(string))
				}
				return true
			})
		}
	}
}

func (cm *ConnectionManager) GetOrCreateConnection(key string, connectionAddress string) (network.Connection, error) {
	// Kiểm tra cache trước
	if cachedConn, ok := cm.connections.Load(key); ok {
		conn := cachedConn.(network.Connection)
		logger.Info("IsConnect status for cached connection: %v", conn.IsConnect())
		if conn.IsConnect() {
			return conn, nil
		}
		cm.connections.Delete(key)
		logger.Warn("Connection lost, removed from cache", "key", key)
	}
	// Tạo connection mới
	conn := pkg_network.NewConnection(
		common.Address{},
		"cluster",
		nil,
	)
	conn.SetRealConnAddr(connectionAddress)
	// Connect
	err := conn.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", key, connectionAddress, err)
	}
	// Đăng ký connection vào socketServer để quản lý readLoop/writeLoop
	if cm.socketServer != nil {
		go cm.socketServer.HandleConnection(conn)
		logger.Info("Connection created and registered with socket server", "key", key, "address", connectionAddress)
	} else {
		// logger.Warn("___SocketServer is nil, connection not registered", "key", key)
		panic("SocketServer is nil, connection not registered")
	}
	// Lưu vào cache
	cm.connections.Store(key, conn)
	return conn, nil
}

// GetConnection lấy connection đã tồn tại (không tạo mới)
func (cm *ConnectionManager) GetConnection(key string) (network.Connection, bool) {
	if cachedConn, ok := cm.connections.Load(key); ok {
		conn := cachedConn.(network.Connection)
		if conn.IsConnect() {
			return conn, true
		}
		// Connection đã mất, xóa khỏi cache
		cm.connections.Delete(key)
	}
	return nil, false
}

// RemoveConnection xóa connection khỏi manager
func (cm *ConnectionManager) RemoveConnection(key string) {
	if cachedConn, ok := cm.connections.Load(key); ok {
		conn := cachedConn.(network.Connection)
		conn.Disconnect()
		cm.connections.Delete(key)
		logger.Info("Connection removed", "key", key)
	}
}

// CloseAll đóng tất cả connections
func (cm *ConnectionManager) CloseAll() {
	// Close old connections
	cm.connections.Range(func(key, value interface{}) bool {
		conn := value.(network.Connection)
		conn.Disconnect()
		cm.connections.Delete(key)
		return true
	})
	// Close new ConnectionClient instances
	cm.clientConnections.Range(func(key, value interface{}) bool {
		client := value.(*connection_client.ConnectionClient)
		client.Disconnect()
		cm.clientConnections.Delete(key)
		logger.Info("ConnectionClient closed: %s", key.(string))
		return true
	})
	// Close TTL connections
	cm.ttlConnections.Range(func(key, value interface{}) bool {
		entry := value.(*ttlEntry)
		entry.client.Disconnect()
		cm.ttlConnections.Delete(key)
		logger.Info("TTL ConnectionClient closed: %s", key.(string))
		return true
	})
	// Cancel context to stop cleanup goroutine
	if cm.cancel != nil {
		cm.cancel()
	}
}

// GetConnectionCount trả về số lượng connections đang active
func (cm *ConnectionManager) GetConnectionCount() int {
	count := 0
	cm.connections.Range(func(key, value interface{}) bool {
		conn := value.(network.Connection)
		if conn.IsConnect() {
			count++
		}
		return true
	})

	cm.clientConnections.Range(func(key, value interface{}) bool {
		client := value.(*connection_client.ConnectionClient)
		if client.IsConnected() {
			count++
		}
		return true
	})

	cm.ttlConnections.Range(func(key, value interface{}) bool {
		entry := value.(*ttlEntry)
		if entry.client.IsConnected() {
			count++
		}
		return true
	})

	return count
}

// GetClientConnectionCount returns the number of active ConnectionClient instances
func (cm *ConnectionManager) GetClientConnectionCount() int {
	count := 0
	cm.clientConnections.Range(func(key, value interface{}) bool {
		client := value.(*connection_client.ConnectionClient)
		if client.IsConnected() {
			count++
		}
		return true
	})
	return count
}

// GetTTLConnectionCount returns the number of active TTL-based ConnectionClient instances
func (cm *ConnectionManager) GetTTLConnectionCount() int {
	count := 0
	cm.ttlConnections.Range(func(key, value interface{}) bool {
		entry := value.(*ttlEntry)
		if entry.client.IsConnected() {
			count++
		}
		return true
	})
	return count
}
