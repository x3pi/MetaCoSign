package config

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"

	tcp_config "github.com/meta-node-blockchain/meta-node/cmd/rpc-client/client-tcp/config"
)

type Config struct {
	RPCServerURL            string   `json:"rpc_server_url"`
	WSSServerURL            string   `json:"wss_server_url"`
	ReadonlyRPCServerURL    string   `json:"readonly_rpc_server_url"`
	ReadonlyWSSServerURL    string   `json:"readonly_wss_server_url"`
	PrivateKey              string   `json:"private_key"`
	ServerPort              string   `json:"server_port"`
	HTTPSPort               string   `json:"https_port"`
	CertFile                string   `json:"cert_file"`
	KeyFile                 string   `json:"key_file"`
	ChainId                 *big.Int `json:"chain_id"`
	MasterPassword          string   `json:"master_password"`
	AppPepper               string   `json:"app_pepper"`
	LdbBlsWalletsPath       string   `json:"ldb_bls_wallet_path"`
	LdbNotificationPath     string   `json:"ldb_bls_account_noti"`
	LdbContractFreeGasPath  string   `json:"ldb_contract_free_gas"`
	LdbArtifactRegistryPath string   `json:"ldb_artifact_registry"`
	LdbRobotTransactionPath string   `json:"ldb_robot_transaction"` // Path cho transaction storage
	OwnerRpcAddress         string   `json:"owner_rpc_address"`
	ContractsInterceptor    []string `json:"contracts_interceptor"` // Địa chỉ contract dùng để intercept
	RewardAmount            *big.Int `json:"reward_amount"`         // Số lượng reward cho mỗi giao dịch được intercept
}

func Load(path string, tcpCfgPath string) (*Config, *tcp_config.ClientConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open config file %s: %w", path, err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	tcpCfg, err := tcp_config.LoadConfig(tcpCfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load tcp config from %s: %w", tcpCfgPath, err)
	}

	clientTcpCfg, ok := tcpCfg.(*tcp_config.ClientConfig)
	if !ok {
		return nil, nil, fmt.Errorf("invalid config type loaded from %s", tcpCfgPath)
	}

	var rawConfig map[string]interface{}
	if err := json.Unmarshal(content, &rawConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config file %s as JSON: %w", path, err)
	}

	config := &Config{}
	if err := json.Unmarshal(content, config); err != nil {
		return nil, nil, fmt.Errorf("failed to decode config file %s into struct: %w", path, err)
	}

	if chainIdVal, ok := rawConfig["chain_id"]; ok {
		switch v := chainIdVal.(type) {
		case float64:
			config.ChainId = big.NewInt(int64(v))
		case string:
			if chainIdInt, success := new(big.Int).SetString(v, 0); success {
				config.ChainId = chainIdInt
			} else {
				return nil, nil, fmt.Errorf("invalid chain_id string format '%s'", v)
			}
		default:
			return nil, nil, fmt.Errorf("invalid type for chain_id (%T)", chainIdVal)
		}
	}

	if err := config.Validate(); err != nil {
		return nil, nil, err
	}
	return config, clientTcpCfg, nil
}

func (c *Config) Validate() error {
	if c.RPCServerURL == "" {
		return fmt.Errorf("missing 'rpc_server_url' in config")
	}
	if c.ChainId == nil {
		return fmt.Errorf("'chain_id' is missing or invalid in config")
	}
	if c.PrivateKey == "" {
		return fmt.Errorf("node's BLS private key ('private_key') is missing in config")
	}
	return nil
}
