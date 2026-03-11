package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	e_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	client_tcp "github.com/meta-node-blockchain/meta-node/cmd/observer/client-tcp"
	tcp_config "github.com/meta-node-blockchain/meta-node/cmd/observer/client-tcp/config"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	pb "github.com/meta-node-blockchain/meta-node/pkg/proto"
	"google.golang.org/protobuf/proto"
)

// ===================== HELPERS =====================

// waitReceipt đợi receipt với timeout 30s, retry 10ms
func waitReceipt(tcpClient *client_tcp.Client, txHash string) *pb.RpcReceipt {
	if txHash == "" {
		return nil
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		receipt, err := tcpClient.RpcGetTransactionReceipt(txHash)
		if err != nil {
			fmt.Printf("  ❌ Receipt error: %v\n", err)
			return nil
		}
		if receipt != nil {
			fmt.Printf("  ✅ Receipt: status=%s, gasUsed=%s, logs=%d\n",
				receipt.Status, receipt.GasUsed, len(receipt.Logs))
			return receipt
		}
		select {
		case <-timer.C:
			fmt.Println("  ⚠️ Timeout waiting for receipt")
			return nil
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// sendTxAndWait tạo, ký, gửi transaction + đợi receipt
func sendTxAndWait(
	tcpClient *client_tcp.Client,
	privKey *ecdsa.PrivateKey,
	fromAddr common.Address,
	toAddr common.Address,
	parsedABI abi.ABI,
	signer e_types.Signer,
	method string,
	args ...interface{},
) (string, *pb.RpcReceipt) {
	nonce, err := tcpClient.RpcGetPendingNonce(fromAddr)
	if err != nil {
		fmt.Printf("  ❌ GetNonce: %v\n", err)
		return "", nil
	}

	inputData, err := parsedABI.Pack(method, args...)
	if err != nil {
		fmt.Printf("  ❌ Pack %s: %v\n", method, err)
		return "", nil
	}

	tx := e_types.NewTransaction(nonce, toAddr, big.NewInt(0), 20000000, big.NewInt(10000000), inputData)
	signedTx, _ := e_types.SignTx(tx, signer, privKey)
	rawTxBytes, _ := signedTx.MarshalBinary()
	rawTxHex := "0x" + hex.EncodeToString(rawTxBytes)

	txHash, err := tcpClient.RpcSendRawTransaction(rawTxHex)
	if err != nil {
		fmt.Printf("  ❌ SendTx %s: %v\n", method, err)
		return "", nil
	}
	fmt.Printf("  ✅ txHash: %s\n", txHash)

	receipt := waitReceipt(tcpClient, txHash)
	return txHash, receipt
}

// makeEventHandler tạo callback log event chung
func makeEventHandler(eventName string, wg *sync.WaitGroup, once *sync.Once) func([]byte) {
	return func(eventData []byte) {
		event := &pb.RpcEvent{}
		if err := proto.Unmarshal(eventData, event); err != nil {
			fmt.Printf("  ❌ Failed to parse RpcEvent: %v\n", err)
			return
		}

		fmt.Printf("\n  📡 [%s] EVENT RECEIVED!\n", eventName)
		fmt.Printf("  ├─ SubscriptionID: %s\n", event.SubscriptionId)
		if event.Log != nil {
			fmt.Printf("  ├─ Contract:       %s\n", event.Log.Address)
			fmt.Printf("  ├─ BlockNumber:    %s\n", event.Log.BlockNumber)
			fmt.Printf("  ├─ TxHash:         %s\n", event.Log.TransactionHash)
			fmt.Printf("  ├─ Topics:         %v\n", event.Log.Topics)
			fmt.Printf("  └─ Data:           %s\n", event.Log.Data)
		}

		if once != nil && wg != nil {
			once.Do(func() {
				wg.Done()
			})
		}
	}
}

// ===================== TEST SUITES =====================

// testDemoContract test các function cơ bản: subscribe events, setValue, increaseValue, verify
func testDemoContract(
	tcpClient *client_tcp.Client,
	demoABI abi.ABI,
	contractAddr common.Address,
	privKey *ecdsa.PrivateKey,
	fromAddr common.Address,
	signer e_types.Signer,
) {
	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  TEST: Demo Contract (setValue + increaseValue)      ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	// 1. Read current value
	fmt.Println("\n─── getValue (trước) ───")
	getValueData, _ := demoABI.Pack("getValue")
	resultBytes, err := tcpClient.RpcEthCall(contractAddr, getValueData)
	if err != nil {
		fmt.Printf("  ❌ %v\n", err)
		return
	}
	results, _ := demoABI.Unpack("getValue", resultBytes)
	oldValue := results[0].(*big.Int)
	fmt.Printf("  ✅ getValue() = %s\n", oldValue.String())

	// 2. Subscribe 2 events
	fmt.Println("\n─── Subscribe ValueChanged + ValueIncreased ───")
	var wgChanged, wgIncreased sync.WaitGroup
	var onceChanged, onceIncreased sync.Once
	wgChanged.Add(1)
	wgIncreased.Add(1)

	sub1, _ := tcpClient.RpcSubscribe(
		[]string{contractAddr.Hex()},
		[]string{demoABI.Events["ValueChanged"].ID.Hex()},
		makeEventHandler("ValueChanged", &wgChanged, &onceChanged),
	)
	fmt.Printf("  ✅ ValueChanged subID=%s\n", sub1)

	sub2, _ := tcpClient.RpcSubscribe(
		[]string{contractAddr.Hex()},
		[]string{demoABI.Events["ValueIncreased"].ID.Hex()},
		makeEventHandler("ValueIncreased", &wgIncreased, &onceIncreased),
	)
	fmt.Printf("  ✅ ValueIncreased subID=%s\n", sub2)

	// 3. setValue(789) + wait receipt
	fmt.Println("\n─── setValue(789) ───")
	sendTxAndWait(tcpClient, privKey, fromAddr, contractAddr, demoABI, signer, "setValue", big.NewInt(789))

	// 4. increaseValue(100) + wait receipt
	fmt.Println("\n─── increaseValue(100) ───")
	sendTxAndWait(tcpClient, privKey, fromAddr, contractAddr, demoABI, signer, "increaseValue", big.NewInt(100))

	// 5. Wait events
	fmt.Println("\n  ⏳ Waiting for both events (max 10s)...")
	allDone := make(chan struct{})
	go func() {
		wgChanged.Wait()
		wgIncreased.Wait()
		close(allDone)
	}()
	select {
	case <-allDone:
		fmt.Println("  ✅ Both events received!")
	case <-time.After(10 * time.Second):
		fmt.Println("  ⚠️ Timeout")
	}

	// 6. Unsubscribe
	tcpClient.RpcUnsubscribe(sub1)
	tcpClient.RpcUnsubscribe(sub2)

	// 7. Verify value
	fmt.Println("\n─── getValue (sau) ───")
	resultBytes2, _ := tcpClient.RpcEthCall(contractAddr, getValueData)
	results2, _ := demoABI.Unpack("getValue", resultBytes2)
	newValue := results2[0].(*big.Int)
	fmt.Printf("  ✅ getValue() = %s", newValue.String())
	if newValue.Int64() == 889 {
		fmt.Println(" ✅ (789 + 100 = 889)")
	} else {
		fmt.Println()
	}
}

// testBlsRegistration test flow: tạo key → getPublickeyBls → setBlsPublicKey → confirmAccountWithoutSign
func testBlsRegistration(
	tcpClient *client_tcp.Client,
	accountABI abi.ABI,
	accountContract common.Address,
	adminPrivKey *ecdsa.PrivateKey,
	adminAddr common.Address,
	signer e_types.Signer,
) {
	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  TEST: BLS Registration + Confirm                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	// === Step 1: Tạo private key mới ===
	fmt.Println("\n─── Step 1: Tạo ETH private key mới ───")
	newPrivKey, err := crypto.GenerateKey()
	if err != nil {
		fmt.Printf("  ❌ GenerateKey: %v\n", err)
		return
	}
	newAddr := crypto.PubkeyToAddress(newPrivKey.PublicKey)
	newPrivKeyHex := hex.EncodeToString(crypto.FromECDSA(newPrivKey))
	fmt.Printf("  ✅ New address:     %s\n", newAddr.Hex())
	fmt.Printf("  ✅ New private key: %s\n", newPrivKeyHex)

	// === Step 2: getPublickeyBls (eth_call đến RPC server) ===
	fmt.Println("\n─── Step 2: getPublickeyBls (eth_call) ───")
	getBlsData, err := accountABI.Pack("getPublickeyBls")
	if err != nil {
		fmt.Printf("  ❌ Pack getPublickeyBls: %v\n", err)
		return
	}
	blsResult, err := tcpClient.RpcEthCall(accountContract, getBlsData)
	if err != nil {
		fmt.Printf("  ❌ eth_call getPublickeyBls: %v\n", err)
		return
	}
	// Server trả JSON string (double-encoded): hex decode → "0x86d5..." (có quotes)
	// Strip quotes để lấy raw BLS pubkey hex
	serverBlsPubKey := strings.Trim(string(blsResult), "\"")
	// Decode hex → bytes để dùng cho setBlsPublicKey
	blsPubKeyHex := strings.TrimPrefix(serverBlsPubKey, "0x")
	blsPubKey, _ := hex.DecodeString(blsPubKeyHex)
	fmt.Printf("  ✅ Server BLS PublicKey: %s (%d bytes)\n", serverBlsPubKey, len(blsPubKey))

	// === Step 3: Subscribe RegisterBls + setBlsPublicKey ===
	fmt.Println("\n─── Step 3: Subscribe RegisterBls + setBlsPublicKey ───")

	// Subscribe RegisterBls event trước khi gửi tx
	registerBlsTopic := accountABI.Events["RegisterBls"].ID.Hex()
	fmt.Printf("  RegisterBls topic: %s\n", registerBlsTopic)

	var wgRegister sync.WaitGroup
	var onceRegister sync.Once
	wgRegister.Add(1)

	subBls, err := tcpClient.RpcSubscribe(
		[]string{accountContract.Hex()},
		[]string{registerBlsTopic},
		makeEventHandler("RegisterBls", &wgRegister, &onceRegister),
	)
	if err != nil {
		fmt.Printf("  ❌ Subscribe RegisterBls: %v\n", err)
	} else {
		fmt.Printf("  ✅ Subscribe RegisterBls: subID=%s\n", subBls)
	}

	// Gửi setBlsPublicKey với BLS key từ server (interceptor xử lý — không có receipt)
	nonce, _ := tcpClient.RpcGetPendingNonce(newAddr)
	inputData, _ := accountABI.Pack("setBlsPublicKey", blsPubKey)
	tx := e_types.NewTransaction(nonce, accountContract, big.NewInt(0), 20000000, big.NewInt(10000000), inputData)
	signedTx, _ := e_types.SignTx(tx, signer, newPrivKey)
	rawTxBytes, _ := signedTx.MarshalBinary()
	txHash, err := tcpClient.RpcSendRawTransaction("0x" + hex.EncodeToString(rawTxBytes))
	if err != nil {
		fmt.Printf("  ❌ setBlsPublicKey: %v\n", err)
	} else {
		fmt.Printf("  ✅ setBlsPublicKey sent: txHash=%s\n", txHash)
	}

	// Đợi RegisterBls event
	fmt.Println("  ⏳ Waiting for RegisterBls event (max 10s)...")
	regDone := make(chan struct{})
	go func() {
		wgRegister.Wait()
		close(regDone)
	}()
	select {
	case <-regDone:
		fmt.Println("  ✅ RegisterBls event received!")
	case <-time.After(10 * time.Second):
		fmt.Println("  ⚠️ Timeout waiting for RegisterBls event")
	}

	tcpClient.RpcUnsubscribe(subBls)

	// === Step 5: confirmAccountWithoutSign (admin confirm) ===
	fmt.Println("\n─── Step 5: confirmAccountWithoutSign (admin confirm) ───")
	fmt.Printf("  ℹ️  Admin %s confirming %s...\n", adminAddr.Hex(), newAddr.Hex())

	txHash2, receipt2 := sendTxAndWait(
		tcpClient, adminPrivKey, adminAddr, accountContract,
		accountABI, signer,
		"confirmAccountWithoutSign", newAddr,
	)
	if receipt2 != nil {
		fmt.Printf("  ✅ confirmAccountWithoutSign OK: txHash=%s\n", txHash2)
	} else {
		fmt.Println("  ⚠️ confirmAccountWithoutSign — receipt not available (may be intercepted)")
	}

	// === Step 6: Verify mtn_getAccountState ===
	fmt.Println("\n─── Step 6: Verify mtn_getAccountState ───")
	params, _ := json.Marshal([]interface{}{newAddr.Hex(), "latest"})
	resp, err := tcpClient.RpcCustomCall("mtn_getAccountState", params, 10*time.Second)
	if err != nil {
		fmt.Printf("  ❌ mtn_getAccountState: %v\n", err)
	} else {
		// Parse JSON result
		var accountState map[string]interface{}
		if err := json.Unmarshal(resp, &accountState); err != nil {
			fmt.Printf("  ❌ Parse result: %v\n", err)
		} else {
			onChainBls, _ := accountState["publicKeyBls"].(string)
			fmt.Printf("  ✅ On-chain BLS: %s\n", onChainBls)
			fmt.Printf("  ✅ Registered:   %s\n", blsPubKeyHex)
			if onChainBls == blsPubKeyHex {
				fmt.Println("  ✅ BLS public key MATCH!")
			} else {
				fmt.Println("  ⚠️ BLS public key MISMATCH")
			}
			balance, _ := accountState["pendingBalance"].(string)
			fmt.Printf("  ✅ Balance: %s wei\n", balance)
		}
	}

	fmt.Println("\n  ✅ BLS Registration flow completed!")
	fmt.Printf("     New address:     %s\n", newAddr.Hex())
	fmt.Printf("     New private key: %s\n", newPrivKeyHex)
	fmt.Printf("     BLS PubKey:      0x%s\n", blsPubKeyHex)
}

// ===================== MAIN =====================

func main() {
	logger.SetConfig(&logger.LoggerConfig{
		Flag:    logger.FLAG_INFO,
		Outputs: []*os.File{os.Stdout},
	})

	configPath := flag.String("config", "config-test.json", "Path to TCP client config")
	testSuite := flag.String("test", "bls", "Test suite: demo, bls, all")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║       TCP-RPC Test Suite                             ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	cfgRaw, _ := tcp_config.LoadConfig(*configPath)
	cfg := cfgRaw.(*tcp_config.ClientConfig)

	// Khởi tạo client
	tcpClient, err := client_tcp.NewClient(cfg)
	if err != nil {
		logger.Error("Failed to create TCP client: %v", err)
		os.Exit(1)
	}
	time.Sleep(1 * time.Second)

	// Common setup
	ethPrivKey, _ := crypto.HexToECDSA(cfg.EthPrivateKey)
	fromAddr := crypto.PubkeyToAddress(ethPrivKey.PublicKey)
	chainIdBig := big.NewInt(int64(cfg.ChainId))
	signer := e_types.NewEIP155Signer(chainIdBig)

	fmt.Printf("\n  Admin address: %s\n", fromAddr.Hex())
	fmt.Printf("  Chain ID: %d\n", cfg.ChainId)

	// === Test 0: Basic RPC ===
	fmt.Println("\n─── Basic: RpcNetVersion + RpcGetChainId ───")
	netVer, _ := tcpClient.RpcNetVersion()
	chainId, _ := tcpClient.RpcGetChainId()
	fmt.Printf("  ✅ NetVersion: %s, ChainId: %s\n", netVer, chainId)

	// === Run test suites ===
	switch *testSuite {
	case "demo":
		// Load demo ABI
		demoAbiBytes, _ := os.ReadFile(cfg.DemoAbiPath_)
		demoABI, _ := abi.JSON(strings.NewReader(string(demoAbiBytes)))
		contractAddr := common.HexToAddress(cfg.DemoContractAddress)
		testDemoContract(tcpClient, demoABI, contractAddr, ethPrivKey, fromAddr, signer)
	case "bls":
		// Load account ABI
		accountABI, _ := abi.JSON(strings.NewReader(accountAbiJSON))
		accountContract := common.HexToAddress("0x00000000000000000000000000000000D844bb55")
		testBlsRegistration(tcpClient, accountABI, accountContract, ethPrivKey, fromAddr, signer)

	case "all":
		// Demo test
		demoAbiBytes, _ := os.ReadFile(cfg.DemoAbiPath_)
		demoABI, _ := abi.JSON(strings.NewReader(string(demoAbiBytes)))
		contractAddr := common.HexToAddress(cfg.DemoContractAddress)
		testDemoContract(tcpClient, demoABI, contractAddr, ethPrivKey, fromAddr, signer)

		// BLS test
		accountABI, _ := abi.JSON(strings.NewReader(accountAbiJSON))
		accountContract := common.HexToAddress("0x00000000000000000000000000000000D844bb55")
		testBlsRegistration(tcpClient, accountABI, accountContract, ethPrivKey, fromAddr, signer)
	}

	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
	fmt.Println("║       All tests completed!                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
}

// Account ABI (chỉ các function cần dùng)
const accountAbiJSON = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": false, "internalType": "address", "name": "account", "type": "address"},
			{"indexed": false, "internalType": "uint256", "name": "time", "type": "uint256"},
			{"indexed": false, "internalType": "bytes", "name": "publicKey", "type": "bytes"},
			{"indexed": false, "internalType": "string", "name": "message", "type": "string"}
		],
		"name": "RegisterBls",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": false, "internalType": "address", "name": "account", "type": "address"},
			{"indexed": false, "internalType": "uint256", "name": "time", "type": "uint256"},
			{"indexed": false, "internalType": "string", "name": "message", "type": "string"}
		],
		"name": "AccountConfirmed",
		"type": "event"
	},
	{
		"inputs": [{"internalType":"bytes","name":"_publicKey","type":"bytes"}],
		"name": "setBlsPublicKey",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"internalType":"address","name":"_account","type":"address"}],
		"name": "confirmAccountWithoutSign",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getPublickeyBls",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
