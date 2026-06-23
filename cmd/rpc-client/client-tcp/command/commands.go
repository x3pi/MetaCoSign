package command

const (
	//General
	InitConnection = "InitConnection"
	Ping           = "Ping"

	GetStats       = "GetStats"
	Stats          = "Stats"
	ChangeLogLevel = "ChangeLogLevel"

	// Send messages
	ReadTransaction              = "ReadTransaction"
	SendTransaction              = "SendTransaction"
	SendTransactionWithDeviceKey = "SendTransactionWithDeviceKey"
	SendTransactions             = "SendTransactions"
	GetAccountState              = "GetAccountState"
	SubscribeToAddress           = "SubscribeToAddress"
	GetStakeState                = "GetStakeState"
	GetSmartContractData         = "GetSmartContractData"
	GetNonce                     = "GetNonce"

	GetDeviceKey = "GetDeviceKey"

	// Receive message
	Nonce             = "Nonce"
	AccountState      = "AccountState"
	StakeState        = "StakeState"
	Receipt           = "Receipt"
	TransactionError  = "TransactionError"
	EventLogs         = "EventLogs"
	QueryLogs         = "QueryLogs"
	SmartContractData = "SmartContractData"
	DeviceKey         = "DeviceKey"

	ServerBusy = "ServerBusy"

	// Chain-direct responses
	ChainId            = "ChainId"
	TransactionReceipt = "TransactionReceipt"
	BlockNumber        = "BlockNumber"
	Logs               = "Logs"
	TransactionByHash  = "TransactionByHash"
	TransactionSuccess = "TransactionSuccess"

	// RPC
	RpcResponse = "RpcResponse"
	RpcEvent    = "RpcEvent"
	GetChainId = "GetChainId"
	GetBlockNumber = "GetBlockNumber"
	GetTransactionReceipt = "GetTransactionReceipt"
	GetLogs = "GetLogs"
)
