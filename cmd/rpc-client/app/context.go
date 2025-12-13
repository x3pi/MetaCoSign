package app

import (
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"
	client_tcp "github.com/meta-node-blockchain/meta-node/cmd/rpc-client/client-tcp"
	tcp_config "github.com/meta-node-blockchain/meta-node/cmd/rpc-client/client-tcp/config"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/config"
	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/internal/ws_interceptor"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/meta-node-blockchain/meta-node/cmd/rpc-client/store"
	"github.com/meta-node-blockchain/meta-node/pkg/bls"
	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/ldb_storage"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	"github.com/meta-node-blockchain/meta-node/pkg/rpc_client"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

type Context struct {
	// RPC Client
	ClientRpc *rpc_client.ClientRPC

	// Private Key Store
	PKS *store.PrivateKeyStore

	// TCP Client
	ClientTcp *client_tcp.Client

	// Configurations
	Cfg    *config.Config
	TcpCfg *tcp_config.ClientConfig

	// LevelDB Storage for BLS wallets
	LdbBlsWallet    *ldb_storage.LevelDBStorage
	LdbNotification *storage.NotificationStorage

	// Contract Free Gas Storage
	LdbContractFreeGas *storage.ContractFreeGasStorage

	// Node BLS keys
	NodeBlsPrivateKey common.PrivateKey
	NodeBlsPublicKey  common.PublicKey

	SubInterceptor *ws_interceptor.SubscriptionInterceptor
}

// New tạo Application Context với tất cả dependencies
func New(cfg *config.Config, tcpCfg *tcp_config.ClientConfig) (*Context, error) {
	// 1. Initialize Private Key Store
	pkStore, err := store.NewPrivateKeyStore(cfg.MasterPassword, cfg.AppPepper)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PrivateKeyStore: %w", err)
	}
	// 2. Load Node BLS key
	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("node's BLS private key is missing in config")
	}
	keyPair := bls.NewKeyPair(ethCommon.FromHex(cfg.PrivateKey))
	// 3. Create RPC client
	clientRpc, err := rpc_client.NewClientRPC(cfg.RPCServerURL, cfg.WSSServerURL, cfg.PrivateKey, cfg.ChainId)
	if err != nil {
		pkStore.Close()
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}
	clientRpc.ChainId = cfg.ChainId
	// 4. Create TCP client
	clientTcp, err := client_tcp.NewClient(tcpCfg)
	if err != nil {
		pkStore.Close()
		return nil, fmt.Errorf("failed to create TCP client: %w", err)
	}
	// 5. Initialize LevelDB for BLS wallets
	ldbBlsWallets, err := ldb_storage.NewLevelDBStorage(cfg.LdbBlsWalletsPath)
	if err != nil {
		pkStore.Close()
		return nil, fmt.Errorf("failed to initialize LevelDB: %w", err)
	}
	db, err := leveldb.OpenFile(cfg.LdbNotificationPath, nil)
	if err != nil {
		return nil, fmt.Errorf("lỗi mở LevelDB tại '%s': %w", cfg.LdbNotificationPath, err)
	}
	ldbNotification := storage.NewNotificationStorage(db)

	// 6. Initialize LevelDB for Contract Free Gas
	ldbContractFreeGas, err := ldb_storage.NewLevelDBStorage(cfg.LdbContractFreeGasPath)
	if err != nil {
		pkStore.Close()
		ldbBlsWallets.Close()
		return nil, fmt.Errorf("failed to initialize Contract Free Gas LevelDB: %w", err)
	}
	contractFreeGasStorage := storage.NewContractFreeGasStorage(ldbContractFreeGas)
	contractFreeGasStorage.AddContract(ethCommon.HexToAddress(cfg.ContractsInterceptor[0]))
	subInterceptor := ws_interceptor.NewSubscriptionInterceptor(cfg)
	ctx := &Context{
		ClientRpc:          clientRpc,
		PKS:                pkStore,
		ClientTcp:          clientTcp,
		Cfg:                cfg,
		TcpCfg:             tcpCfg,
		LdbBlsWallet:       ldbBlsWallets,
		LdbNotification:    ldbNotification,
		LdbContractFreeGas: contractFreeGasStorage,
		NodeBlsPrivateKey:  keyPair.PrivateKey(),
		NodeBlsPublicKey:   keyPair.PublicKey(),
		SubInterceptor:     subInterceptor,
	}
	return ctx, nil
}

// Close đóng tất cả resources
func (ctx *Context) Close() error {
	logger.Info("Closing application context...")

	var errors []error

	if ctx.PKS != nil {
		if err := ctx.PKS.Close(); err != nil {
			logger.Error("Failed to close PrivateKeyStore: %v", err)
			errors = append(errors, err)
		}
	}

	if ctx.LdbBlsWallet != nil {
		ctx.LdbBlsWallet.Close()
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during context close: %v", errors)
	}
	return nil
}
