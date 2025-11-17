package node

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	// Đảm bảo đã import
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

type ConnectionStatus int

const (
	Disconnected ConnectionStatus = iota
	Connecting
	Connected
	Failed
)
const (
	BackupStorageKey = "backup"
)

func (s ConnectionStatus) String() string {
	switch s {
	case Disconnected:
		return "Disconnected"
	case Connecting:
		return "Connecting"
	case Connected:
		return "Connected"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

type PeerInfo struct {
	Info           peer.AddrInfo
	Type           string
	Status         ConnectionStatus
	LastConnected  time.Time
	ReconnectCount int
	LastError      string
}
type PublishJob struct {
	TopicName string
	Data      []byte
	Ctx       context.Context
}

// HostNode là đối tượng chứa libp2p host, danh sách subscriber và peer theo loại
type HostNode struct {
	Host                host.Host
	NodeType            string
	Subscribers         map[string]*pubsub.Topic
	PubSub              *pubsub.PubSub
	Peers               map[string]*PeerInfo
	rootPath            string
	KeyValueStore       *lru.Cache[string, []byte]
	wg                  sync.WaitGroup
	reconnectMutex      sync.Mutex
	ctx                 context.Context
	cancelReconnect     map[string]context.CancelFunc
	cancelMutex         sync.Mutex
	TopicStorageMap     sync.Map
	fetchingBlocks      sync.Map
	TransactionChan     chan []byte
	ReadTransactionChan chan []byte

	// SỬA ĐỔI: Thêm kênh chuyên dụng để xử lý block, tách biệt với mạng
	BlockProcessingQueue chan *storage.BackUpDb

	FeeAddresses    []string
	topicStorageMap map[string]storage.Storage
	publishQueue    chan PublishJob
}

func NewHostNodeWithConfig(ctx context.Context, privKey crypto.PrivKey, listenPort int, rootPath string, nodeType string, config Config) (*HostNode, error) {
	// ... (Phần khởi tạo libp2p host, pubsub, cache giữ nguyên)
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", listenPort)
	tcpListenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)
	cm, err := connmgr.NewConnManager(config.MinConnections, config.MaxConnections, connmgr.WithGracePeriod(config.GracePeriod))
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}
	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(listenAddr, tcpListenAddr),
		libp2p.ConnectionManager(cm),
		libp2p.EnableRelayService(),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}
	const newMaxMessageSize = 2048 * 1024 * 1024
	psOptions := []pubsub.Option{pubsub.WithMaxMessageSize(newMaxMessageSize)}
	ps, err := pubsub.NewGossipSub(ctx, h, psOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}
	const maxCacheSize = 100000
	cache, err := lru.New[string, []byte](maxCacheSize)
	if err != nil {
		log.Fatalf("Không thể tạo LRU cache: %v", err)
	}

	const publishQueueSize = 5000000
	const blockProcessingQueueSize = 2000000 // Hàng đợi xử lý block không cần quá lớn
	node := &HostNode{
		Host:                h,
		NodeType:            nodeType,
		Subscribers:         make(map[string]*pubsub.Topic),
		PubSub:              ps,
		Peers:               make(map[string]*PeerInfo),
		rootPath:            rootPath,
		ctx:                 ctx,
		cancelReconnect:     make(map[string]context.CancelFunc),
		KeyValueStore:       cache,
		TransactionChan:     make(chan []byte, 2000000),
		ReadTransactionChan: make(chan []byte, 2000000),

		// SỬA ĐỔI: Khởi tạo kênh mới
		BlockProcessingQueue: make(chan *storage.BackUpDb, blockProcessingQueueSize),

		topicStorageMap: make(map[string]storage.Storage),
		publishQueue:    make(chan PublishJob, publishQueueSize),
	}

	// ... (Phần còn lại của hàm NewHostNodeWithConfig giữ nguyên)
	go node.publishWorker()
	node.setupConnectionNotifier(config)
	node.startPeerHealthMonitor(config.PingInterval, config.PingTimeout)
	if node.NodeType == "sub" {
		node.Host.SetStreamHandler(BlockRequestProtocol, node.blockRequestHandler)
		logger.Info(fmt.Sprintf("Set stream handler for %s (Node Type: %s)", BlockRequestProtocol, node.NodeType))
	} else {
		logger.Info(fmt.Sprintf("Skipping block request handler setup (Node Type: %s)", node.NodeType))
	}
	go func() {
		nodeCtx := node.ctx
		if nodeCtx == nil {
			nodeCtx = context.Background()
		}
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		logger.Info("Started periodic cleanup task for stale receiving states.")
		for {
			select {
			case <-ticker.C:
				logger.Debug("Running periodic cleanup for receiving states...")
				CleanupOldStates(24 * time.Hour)
			case <-nodeCtx.Done():
				logger.Info("Stopping receiving state cleanup task.")
				return
			}
		}
	}()
	node.DisplayNodeInfo()
	return node, nil
}

// ... (phần còn lại của file giữ nguyên không thay đổi)
// SetFeeAddresses cập nhật danh sách địa chỉ phí một cách an toàn.
func (node *HostNode) SetFeeAddresses(addresses []string) {
	// Tạo bản sao để tránh sửa đổi slice ban đầu từ bên ngoài
	node.FeeAddresses = make([]string, len(addresses))
	copy(node.FeeAddresses, addresses)
	logger.Debug(fmt.Sprintf("Updated FeeAddresses: %v", node.FeeAddresses))
}

// GetFeeAddresses trả về bản sao của danh sách địa chỉ phí hiện tại một cách an toàn.
func (node *HostNode) GetFeeAddresses() []string {
	// Trả về bản sao để tránh sửa đổi slice nội bộ từ bên ngoài
	addressesCopy := make([]string, len(node.FeeAddresses))
	copy(addressesCopy, node.FeeAddresses)
	return addressesCopy
}

// AddTopicStorage thêm hoặc cập nhật một storage instance cho một topic cụ thể.
func (node *HostNode) AddTopicStorage(topicName string, storageInstance storage.Storage) {
	if storageInstance == nil {
		logger.Warn(fmt.Sprintf("Attempted to add nil storage for topic: %s", topicName))
		return // Hoặc trả về lỗi
	}
	// Sử dụng Store của sync.Map
	node.TopicStorageMap.Store(topicName, storageInstance)
	logger.Info(fmt.Sprintf("Added/Updated storage for topic: %s using sync.Map", topicName))
}

// RemoveTopicStorage xóa storage instance liên kết với một topic khỏi map quản lý.
// Lưu ý: Hàm này KHÔNG đóng kết nối của storage instance.
func (node *HostNode) RemoveTopicStorage(topicName string) error {
	// Kiểm tra trước khi xóa (tùy chọn với sync.Map, nhưng hữu ích để báo lỗi)
	if _, loaded := node.TopicStorageMap.Load(topicName); !loaded {
		return fmt.Errorf("storage for topic '%s' not found in sync.Map", topicName)
	}
	// Sử dụng Delete của sync.Map
	node.TopicStorageMap.Delete(topicName)
	logger.Info(fmt.Sprintf("Removed storage reference for topic: %s using sync.Map", topicName))
	return nil
}

// GetTopicStorage lấy storage instance liên kết với một topic một cách an toàn.
func (node *HostNode) GetTopicStorage(topicName string) (storage.Storage, bool) {
	// Sử dụng Load của sync.Map
	value, loaded := node.TopicStorageMap.Load(topicName)
	if !loaded {
		return nil, false // Không tìm thấy
	}
	// Cần thực hiện type assertion vì Load trả về any (interface{})
	storageInstance, ok := value.(storage.Storage)
	if !ok {
		// Lỗi lập trình: đã lưu trữ sai kiểu dữ liệu vào map
		logger.Error(fmt.Sprintf("Invalid type stored in sync.Map for topic %s: expected storage.Storage, got %T", topicName, value))
		return nil, false
	}
	return storageInstance, true
}

// SetTopicStorageMap gán một map[string]storage.Storage vào trường topicStorageMap của HostNode.
func (node *HostNode) SetTopicStorageMap(m map[string]storage.Storage) {
	node.topicStorageMap = m
}

// Config holds configuration parameters for the HostNode
type Config struct {
	MinConnections        int           // Minimum number of connections to maintain
	MaxConnections        int           // Maximum number of connections allowed
	GracePeriod           time.Duration // Grace period for connection manager
	InitialReconnectDelay time.Duration // Initial delay before reconnection attempt
	MaxReconnectDelay     time.Duration // Maximum delay between reconnection attempts
	MaxReconnectAttempts  int           // Maximum number of reconnection attempts
	PingInterval          time.Duration // How often to ping peers to check liveliness
	PingTimeout           time.Duration // Timeout for ping responses
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MinConnections:        100,
		MaxConnections:        400,
		GracePeriod:           time.Second,
		InitialReconnectDelay: 50 * time.Millisecond,
		MaxReconnectDelay:     2 * time.Minute,
		MaxReconnectAttempts:  100, // Effectively unlimited for important peers
		PingInterval:          3 * time.Second,
		PingTimeout:           1 * time.Second,
	}
}

// LoadPrivateKey giải mã private key từ chuỗi base64
func LoadPrivateKey(keyStr string) (crypto.PrivKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}

	privKey, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}
	return privKey, nil
}

func NewHostNode(ctx context.Context, privKey crypto.PrivKey, listenPort int, rootPath string, nodeType string) (*HostNode, error) {
	return NewHostNodeWithConfig(ctx, privKey, listenPort, rootPath, nodeType, DefaultConfig())
}

func (node *HostNode) DisplayNodeInfo() {
	// Hiển thị thông tin node
	fmt.Println("Node ID:", node.Host.ID().String())
	fmt.Println("Node lắng nghe tại các địa chỉ:")
	for _, addr := range node.Host.Addrs() {
		fmt.Printf("%s/p2p/%s\n", addr, node.Host.ID().String())
	}
}
func (node *HostNode) AddPeer(peerAddr string, peerType string) {
	node.reconnectMutex.Lock()
	defer node.reconnectMutex.Unlock()
	addr, err := peer.AddrInfoFromString(peerAddr)
	if err != nil {
		logger.Error("không thể phân tích địa chỉ peer: %w", err)
	}

	node.Peers[addr.ID.String()] = &PeerInfo{
		Info:           *addr,
		Type:           peerType,
		Status:         Disconnected, // Ban đầu chưa kết nối
		ReconnectCount: 0,
	}
}

// CheckConnection kiểm tra kết nối với peer
func (node *HostNode) CheckConnection(peerAddr string) (bool, error) {
	addr, err := peer.AddrInfoFromString(peerAddr)
	if err != nil {
		return false, fmt.Errorf("không thể phân tích địa chỉ peer: %w", err)
	}

	// Kiểm tra xem peer đã được kết nối hay chưa
	peerInfo, ok := node.Peers[addr.ID.String()]
	if ok && peerInfo.Status == Connected {
		// Validate the connection is still active
		if node.Host.Network().Connectedness(addr.ID) == network.Connected {
			return true, nil
		}

		// Connection is not actually active, update status
		peerInfo.Status = Disconnected
		return false, nil
	}

	return false, nil
}

// Automatically retry connections to important peers with exponential backoff
func (node *HostNode) reconnectToPeer(peerID string, initialDelay, maxDelay time.Duration, maxAttempts int) {
	node.cancelMutex.Lock()
	// Cancel any existing reconnection attempt for this peer
	if cancel, exists := node.cancelReconnect[peerID]; exists {
		cancel()
	}

	// Create a new cancellable context for this reconnection process
	ctx, cancel := context.WithCancel(node.ctx)
	node.cancelReconnect[peerID] = cancel
	node.cancelMutex.Unlock()

	go func() {
		defer func() {
			node.cancelMutex.Lock()
			delete(node.cancelReconnect, peerID)
			node.cancelMutex.Unlock()
		}()

		peerInfo, exists := node.Peers[peerID]
		if !exists {
			logger.Error("Cannot reconnect to unknown peer:", peerID)
			return
		}

		// Implement exponential backoff for reconnection attempts
		delay := initialDelay

		for attempt := 0; attempt < maxAttempts; attempt++ {
			// Check if context is cancelled or we're already connected
			if ctx.Err() != nil {
				return
			}

			if node.Host.Network().Connectedness(peerInfo.Info.ID) == network.Connected {
				peerInfo.Status = Connected
				peerInfo.LastConnected = time.Now()
				peerInfo.ReconnectCount = 0
				logger.Info("✅ Successfully reconnected to peer:", peerID)
				return
			}

			// Update peer status
			node.reconnectMutex.Lock()
			peerInfo.Status = Connecting
			peerInfo.ReconnectCount++
			node.reconnectMutex.Unlock()

			logger.Info(fmt.Sprintf("🔄 Attempting to reconnect to peer %s (attempt %d/%d)", peerID, attempt+1, maxAttempts))

			// Try to connect
			err := node.Host.Connect(ctx, peerInfo.Info)
			if err == nil {
				node.reconnectMutex.Lock()
				peerInfo.Status = Connected
				peerInfo.LastConnected = time.Now()
				peerInfo.ReconnectCount = 0
				peerInfo.LastError = ""
				node.reconnectMutex.Unlock()
				logger.Info("✅ Successfully reconnected to peer:", peerID)
				return
			}

			// Update failure info
			node.reconnectMutex.Lock()
			peerInfo.Status = Failed
			peerInfo.LastError = err.Error()
			node.reconnectMutex.Unlock()

			logger.Warn(fmt.Sprintf("❌ Failed to reconnect to peer %s: %v", peerID, err))

			// Wait before next attempt with exponential backoff
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
				// Exponential backoff with jitter
				jitter := time.Duration(float64(delay) * 0.1 * (1.0 + rand.Float64() - 0.5))
				delay = time.Duration(math.Min(
					float64(maxDelay),
					float64(delay)*1.5,
				)) + jitter
			}
		}

		logger.Error(fmt.Sprintf("⛔ Giving up reconnection to peer %s after %d attempts", peerID, maxAttempts))
	}()
}

func (node *HostNode) HandleIncomingMessages(ctx context.Context, topicName string, addressMaster string, topicStorageMap map[string]storage.Storage, resync bool) error {
	topic, exists := node.Subscribers[topicName]
	if !exists {
		return fmt.Errorf("chưa đăng ký topic '%s'", topicName)
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("không thể tạo subscription cho topic '%s': %w", topicName, err)
	}

	go func() {
		for {
			msg, err := subscription.Next(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					logger.Info(fmt.Sprintf("Subscription context for topic '%s' đã đóng", topicName))
					return
				}
				logger.Error(fmt.Sprintf("Lỗi khi nhận tin nhắn từ topic '%s': %s", topicName, err))
				time.Sleep(100 * time.Millisecond) // Tránh vòng lặp vô tận khi lỗi liên tục xảy ra
				continue
			}

			if err := node.processMessage(topicName, topicStorageMap, msg, resync); err != nil {
				logger.Error(fmt.Sprintf("Lỗi khi xử lý tin nhắn từ topic '%s': %v", topicName, err))
			}
		}
	}()

	fmt.Printf("Đang lắng nghe dữ liệu từ topic '%s'\n", topicName)
	return nil
}

// ConnectToPeer connects to a peer with retry logic
func (node *HostNode) ConnectToPeer(ctx context.Context, peerAddr, peerType string) error {
	addr, err := peer.AddrInfoFromString(peerAddr)
	if err != nil {
		return fmt.Errorf("không thể phân tích địa chỉ peer: %w", err)
	}

	// Check if we're already trying to connect to this peer
	node.reconnectMutex.Lock()
	if peerInfo, exists := node.Peers[addr.ID.String()]; exists && peerInfo.Status == Connecting {
		node.reconnectMutex.Unlock()
		logger.Info(fmt.Sprintf("Already connecting to peer %s, waiting for result", addr.ID.String()))

		// Wait a bit to see if the connection succeeds
		startTime := time.Now()
		for time.Since(startTime) < 10*time.Second {
			if node.Host.Network().Connectedness(addr.ID) == network.Connected {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Continue with connection attempt if we're still not connected
	} else {
		// Add peer to our map before attempting connection
		node.Peers[addr.ID.String()] = &PeerInfo{
			Info:           *addr,
			Type:           peerType,
			Status:         Connecting,
			ReconnectCount: 0,
		}
		node.reconnectMutex.Unlock()
	}

	// Add peer to peerstore with permanent TTL for address retention
	node.Host.Peerstore().AddAddrs(addr.ID, addr.Addrs, peerstore.PermanentAddrTTL)

	const maxRetries = 5
	const retryDelay = 3 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Info(fmt.Sprintf("Connecting to peer %s (attempt %d/%d)", peerAddr, attempt, maxRetries))

		err := node.Host.Connect(ctx, *addr)
		if err != nil {
			logger.Error(fmt.Sprintf("Lỗi khi kết nối tới peer (lần %d/%d): %s\n", attempt, maxRetries, err))

			node.reconnectMutex.Lock()
			if peerInfo, exists := node.Peers[addr.ID.String()]; exists {
				peerInfo.Status = Failed
				peerInfo.LastError = err.Error()
			}
			node.reconnectMutex.Unlock()

			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}

			// If all attempts fail, start background reconnection
			config := DefaultConfig()
			node.reconnectToPeer(addr.ID.String(), config.InitialReconnectDelay, config.MaxReconnectDelay, config.MaxReconnectAttempts)

			return fmt.Errorf("không thể kết nối tới peer %s sau %d lần: %w", peerAddr, maxRetries, err)
		}

		// Update peer status on successful connection
		node.reconnectMutex.Lock()
		node.Peers[addr.ID.String()] = &PeerInfo{
			Info:          *addr,
			Type:          peerType,
			Status:        Connected,
			LastConnected: time.Now(),
		}
		node.reconnectMutex.Unlock()

		fmt.Printf("✅ Đã kết nối tới peer %s (Loại: %s)\n", peerAddr, peerType)
		return nil
	}

	return fmt.Errorf("kết nối thất bại sau %d lần thử", maxRetries)
}

// Implement peer health monitoring with periodic pings
func (node *HostNode) startPeerHealthMonitor(interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-node.ctx.Done():
				return
			case <-ticker.C:
				node.checkAllPeerConnections()
			}
		}
	}()
}

// Check all peer connections and attempt to reconnect to disconnected peers
func (node *HostNode) checkAllPeerConnections() {
	var peerIDs []string

	node.reconnectMutex.Lock()
	for peerID := range node.Peers {
		peerIDs = append(peerIDs, peerID)
	}
	node.reconnectMutex.Unlock()

	totalPeers := len(peerIDs)
	connectedPeers := 0

	for _, peerID := range peerIDs {
		peerIDObj, err := peer.Decode(peerID)
		if err != nil {
			log.Printf("Invalid peer ID %s: %v", peerID, err)
			continue
		}

		// Kiểm tra trạng thái kết nối của peer
		if node.Host.Network().Connectedness(peerIDObj) == network.Connected {
			connectedPeers++
		} else {
			node.reconnectMutex.Lock()
			peerInfo, exists := node.Peers[peerID]
			if !exists {
				node.reconnectMutex.Unlock()
				continue
			}

			if peerInfo.Status != Connecting {
				peerInfo.Status = Disconnected
				node.reconnectMutex.Unlock()

				// Bắt đầu quá trình reconnect
				config := DefaultConfig()
				node.reconnectToPeer(peerID, config.InitialReconnectDelay, config.MaxReconnectDelay, config.MaxReconnectAttempts)
			} else {
				node.reconnectMutex.Unlock()
			}
		}
	}

	logger.Info(fmt.Sprintf("🔄 Kết nối: %d/%d peer đã kết nối", connectedPeers, totalPeers))
	storage.UpdateConnectState(1)

	// Kiểm tra nếu tất cả đã kết nối
	// if connectedPeers == totalPeers || totalPeers == 0 {
	// 	logger.Info("✅ Tất cả các peer đã được kết nối!")
	// 	storage.UpdateConnectState(1)
	// }
}

func (node *HostNode) setupConnectionNotifier(config Config) {
	node.Host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			peerID := conn.RemotePeer().String()
			log.Printf("✅ Connected to peer: %s", peerID)

			node.reconnectMutex.Lock()
			defer node.reconnectMutex.Unlock()

			if peerInfo, exists := node.Peers[peerID]; exists {
				peerInfo.Status = Connected
				peerInfo.LastConnected = time.Now()
				peerInfo.ReconnectCount = 0
				peerInfo.LastError = ""
			}
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			peerID := conn.RemotePeer().String()
			log.Printf("❌ Disconnected from peer: %s", peerID)

			node.reconnectMutex.Lock()
			peerInfo, exists := node.Peers[peerID]
			if exists {
				peerInfo.Status = Disconnected
			}
			node.reconnectMutex.Unlock()
			node.reconnectToPeer(peerID, config.InitialReconnectDelay, config.MaxReconnectDelay, config.MaxReconnectAttempts)

			// Don't immediately reconnect here, let the health monitor handle it
			// This prevents connection storms when multiple disconnections happen
		},
	})
}

// Close gracefully shuts down the host node and all goroutines
func (node *HostNode) Close() error {
	// Cancel all reconnection goroutines
	node.cancelMutex.Lock()
	for _, cancel := range node.cancelReconnect {
		cancel()
	}
	node.cancelReconnect = make(map[string]context.CancelFunc)
	node.cancelMutex.Unlock()

	// Wait for any pending operations to complete
	node.wg.Wait()

	// Close the host
	return node.Host.Close()
}

// HandleRequest - Xử lý yêu cầu file từ Node B, chỉ trả về tên file
func (n *HostNode) HandleRequest(stream network.Stream) (string, error) {
	defer stream.Close()

	reader := bufio.NewReader(stream)

	// Đọc tên file từ Node B
	fileName, err := reader.ReadString('\n')
	if err != nil {
		log.Println("❌ Lỗi đọc dữ liệu từ stream:", err)
		return "", err
	}
	fileName = strings.TrimSpace(fileName)
	log.Println("📥 Yêu cầu nhận file:", fileName)

	// Gửi phản hồi "OK" nếu file tồn tại
	stream.Write([]byte("OK\n"))

	// Trả về tên file
	return fileName, nil
}

func (node *HostNode) SendRequestToMaster(ctx context.Context, message string) error {
	logger.Info("Searching for Master peer to send request...")
	var masterPeerAddress string // Biến lưu địa chỉ master tìm được

	// *** LẶP QUA TẤT CẢ PEERS ĐỂ TÌM MASTER ***
	node.reconnectMutex.Lock() // Sử dụng mutex bảo vệ truy cập node.Peers
	for peerStrID, peerInfo := range node.Peers {
		// Kiểm tra xem Type có phải là "master" không
		if peerInfo.Type == "master" {
			logger.Info(fmt.Sprintf("Found master peer: %s", peerStrID))
			// Kiểm tra xem peerInfo có địa chỉ không
			if len(peerInfo.Info.Addrs) > 0 {
				// Lấy địa chỉ đầu tiên và định dạng nó
				masterPeerAddress = fmt.Sprintf("%s/p2p/%s", peerInfo.Info.Addrs[0].String(), peerStrID)
				logger.Info(fmt.Sprintf("Using master peer address: %s", masterPeerAddress))
				break // Đã tìm thấy master đầu tiên, thoát vòng lặp
			} else {
				logger.Warn(fmt.Sprintf("Master peer %s found but has no addresses listed.", peerStrID))
			}
		}
	}
	node.reconnectMutex.Unlock() // Mở khóa mutex sau khi duyệt xong

	// Kiểm tra xem có tìm thấy địa chỉ master không
	if masterPeerAddress == "" {
		logger.Error("❌ Could not find an active Master node address.")
		return errors.New("master peer not found or has no address") // Trả về lỗi cụ thể
	}

	// Gửi yêu cầu bằng cách sử dụng node.SendRequest (giả định nó tồn tại)
	logger.Info(fmt.Sprintf("Sending '%s' request to Master node %s", message, masterPeerAddress))
	err := node.SendRequest(ctx, masterPeerAddress, message) // Gọi SendRequest
	if err != nil {
		logger.Error(fmt.Sprintf("❌ Request '%s' failed to Master %s: %v", message, masterPeerAddress, err))
		return fmt.Errorf("failed to send '%s' request to master %s: %w", message, masterPeerAddress, err)
	}

	logger.Info(fmt.Sprintf("✅ Successfully sent '%s' request to Master.", message))
	return nil // Trả về nil nếu thành công
}
