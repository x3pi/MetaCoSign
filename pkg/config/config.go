package config

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	"github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/pathdetector"
	"github.com/meta-node-blockchain/meta-node/pkg/storage"
)

// NodeType định nghĩa các loại node lưu trữ.
type NodeType string

const (
	// STORAGE_REMOTE chỉ định rằng node sử dụng lưu trữ từ xa.
	STORAGE_REMOTE NodeType = "STORAGE_REMOTE"
	STORAGE_CLIENT NodeType = "STORAGE_CLIENT"

	// STORAGE_LOCAL chỉ định rằng node sử dụng lưu trữ cục bộ.
	STORAGE_LOCAL NodeType = "STORAGE_LOCAL"
)

var (
	ConfigApp  *SimpleChainConfig
	loadConfig sync.Once
)

// DBDetail định nghĩa cấu trúc cho một database cụ thể, bao gồm đường dẫn và địa chỉ lắng nghe.
type DBDetail struct {
	Path          string `json:"Path"`
	ListenAddress string `json:"ListenAddress"` // Đã xóa omitempty, trường này là bắt buộc
	DBEngine      string `json:"DBEngine,omitempty"`
}

// DatabasesConfig định nghĩa cấu trúc cho đối tượng "Databases" trong file JSON.
type DatabasesConfig struct {
	RootPath               string   `json:"RootPath"`
	NodeType               NodeType `json:"NodeType"`
	Version                string   `json:"Version"`
	BLSPrivateKey          string   `json:"BLSPrivateKey"`
	SnapshotPath           string   `json:"SnapshotPath"`
	MaxPartSizeMB          int      `json:"MaxPartSizeMB"`
	ArchiveBaseName        string   `json:"ArchiveBaseName"`
	AccountState           DBDetail `json:"AccountState"`
	Trie                   DBDetail `json:"Trie"`
	SmartContractCode      DBDetail `json:"SmartContractCode"`
	SmartContractStorage   DBDetail `json:"SmartContractStorage"`
	Blocks                 DBDetail `json:"Blocks"`
	Receipts               DBDetail `json:"Receipts"`
	TxsEth                 DBDetail `json:"TxsEth"`
	BlocksHash             DBDetail `json:"BlocksHash"`
	BackupDeviceKey        DBDetail `json:"BackupDeviceKey"`
	TransactionBlockNumber DBDetail `json:"TransactionBlockNumber"`
	TransactionState       DBDetail `json:"TransactionState"`
	BlockHashToNumber      DBDetail `json:"BlockHashToNumber"`
	Wallets                DBDetail `json:"Wallets"`
	Mapping                DBDetail `json:"Mapping"`
	Backup                 DBDetail `json:"Backup"`
	Stake                  DBDetail `json:"Stake"`
}

// NodesConfig khớp với cấu trúc của đối tượng "nodes" trong JSON.
type NodesConfig struct {
	PrivateKey            string   `json:"privateKey"`
	Master                string   `json:"master"`
	ListenPort            int      `json:"listen_port"`
	ListSubNode           []string `json:"list_sub_node"`
	Resync                bool     `json:"resync"`
	MasterAddress         string   `json:"master_address"`
	MasterReadOnlyAddress string   `json:"master_read_only_address"`
	ListSubAddress        []string `json:"list_sub_address"`
	ListExecAddress       []string `json:"list_exec_address"`
}

// SimpleChainConfig là struct chính, đại diện cho toàn bộ file config JSON.
type SimpleChainConfig struct {
	Debug                   bool   `json:"debug"`
	Mode                    string `json:"mode"`
	ExplorereDbPath         string `json:"explorer_db_path"`
	ExplorereReadOnlyDbPath string `json:"explorer_read_only_db_path"`

	IsExplorer          bool `json:"is_explorer"`
	ExplorerQueueSize   int  `json:"explorer_queue_size"`
	ExplorerWorkerCount int  `json:"explorer_worker_count"`

	MiningDbPath           string `json:"mining_db_path"`
	IsMining               bool   `json:"is_mining"`
	ClientRpcUrl           string `json:"client_rpc_url"`
	RewardSenderPrivateKey string `json:"reward_sender_private_key"`
	RewardSenderAddress    string `json:"reward_sender_address"`

	ChainId                            *big.Int           `json:"chainId"`
	PrivateKey                         string             `json:"private_key"`
	Address                            string             `json:"address"`
	LogPath                            string             `json:"log_path"`
	BackupPath                         string             `json:"backup_path"`
	LastBlockSavePath                  string             `json:"last_block_save_path"`
	TransactionBlockNumberLastHashPath string             `json:"transaction_block_number_last_hash_path"`
	BlockHashToNumberDBRootPath        string             `json:"block_hash_to_number_db_root_path"`
	FreeFeeAddresses                   []string           `json:"free_fee_addresses"`
	ConnectionAddress                  string             `json:"connection_address"`
	DNSServerAddress                   string             `json:"dns_server_address"`
	Version                            string             `json:"version"`
	NodeType                           string             `json:"node_type"`
	ListTypeService                    string             `json:"list_type_service"`
	ServiceType                        common.ServiceType `json:"service_type"`
	RpcPort                            string             `json:"rpc_port"`
	DBType                             storage.DBType     `json:"db_type"`
	GenesisFilePath                    string             `json:"genesis_file_path"`
	Securepassword                     string             `json:"securepassword"`
	//
	PkAdminFileStorage      string `json:"pk_admin_file_storage"`
	BlsAdminStorage         string `json:"bls_admin_storage"`
	OwnerFileStorageAddress string `json:"owner_file_storage_address"`

	Databases DatabasesConfig `json:"Databases"`
	Nodes     NodesConfig     `json:"nodes"`
}

// joinPathIfNotURL nối path với base path chỉ khi path không phải là URL.
func JoinPathIfNotURL(basePath, path string) string {
	pathType := pathdetector.DetectPathType(path)
	if pathType == pathdetector.URL {
		return path
	}
	return filepath.Join(basePath, path)
}

// LoadConfig đọc và xử lý file cấu hình.
func LoadConfig(configPath string) (*SimpleChainConfig, error) {
	var err error
	loadConfig.Do(func() {
		ConfigApp = &SimpleChainConfig{}
		var raw []byte
		raw, err = os.ReadFile(configPath)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(raw, ConfigApp)
		if err != nil {
			panic(err)
		}

		if ConfigApp.ExplorerQueueSize == 0 {
			ConfigApp.ExplorerQueueSize = 8192
		}
		if ConfigApp.ExplorerWorkerCount == 0 {
			ConfigApp.ExplorerWorkerCount = 32
		}

	})
	return ConfigApp, err
}
