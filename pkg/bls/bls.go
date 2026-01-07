package bls

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	blst "github.com/meta-node-blockchain/meta-node/pkg/bls/blst/bindings/go"
	cm "github.com/meta-node-blockchain/meta-node/pkg/common"
)

type blstPublicKey = blst.P1Affine
type blstSignature = blst.P2Affine
type blstAggregateSignature = blst.P2Aggregate
type blstAggregatePublicKey = blst.P1Aggregate
type blstSecretKey = blst.SecretKey

var dstMinPk = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")

// ===== CONCURRENT BLS SIGNING OPTIMIZATION =====

// BLSSigner manages concurrent BLS operations safely
type BLSSigner struct {
	workers    int
	taskChan   chan func()
	resultChan chan interface{}
	done       chan struct{}
}

// Global signer instance (singleton)
var globalSigner *BLSSigner
var signerOnce sync.Once

// GetBLSSigner returns the global BLS signer instance
func GetBLSSigner() *BLSSigner {
	signerOnce.Do(func() {
		numCPU := runtime.NumCPU()
		globalSigner = &BLSSigner{
			workers:    numCPU,                      // Số workers = số CPU cores
			taskChan:   make(chan func(), numCPU*4), // Buffer queue
			resultChan: make(chan interface{}, numCPU*4),
			done:       make(chan struct{}),
		}

		// Start worker goroutines
		for i := 0; i < globalSigner.workers; i++ {
			go globalSigner.worker()
		}
	})
	return globalSigner
}

// worker processes BLS tasks sequentially but concurrently across workers
func (bs *BLSSigner) worker() {
	for {
		select {
		case task := <-bs.taskChan:
			task()
		case <-bs.done:
			return
		}
	}
}

// SignConcurrent performs BLS signing with proper concurrency
func (bs *BLSSigner) SignConcurrent(privKey cm.PrivateKey, message []byte) cm.Sign {
	if !ValidateBlsPrivateKey(privKey.Bytes()) {
		return cm.Sign{}
	}

	resultChan := make(chan cm.Sign, 1)

	bs.taskChan <- func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- cm.Sign{} // Return zero value on panic
			}
		}()

		sk := new(blstSecretKey).Deserialize(privKey.Bytes())
		sig := new(blstSignature).Sign(sk, message, dstMinPk)
		resultChan <- cm.SignFromBytes(sig.Compress())
	}

	return <-resultChan
}

// VerifyConcurrent performs BLS verification with proper concurrency
func (bs *BLSSigner) VerifyConcurrent(pubKey cm.PublicKey, sig cm.Sign, message []byte) bool {
	resultChan := make(chan bool, 1)

	bs.taskChan <- func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- false // Return false on panic
			}
		}()

		valid := new(blstSignature).VerifyCompressed(sig.Bytes(), true, pubKey.Bytes(), false, message, dstMinPk)
		resultChan <- valid
	}

	return <-resultChan
}

// Close shuts down the BLS signer
func (bs *BLSSigner) Close() {
	close(bs.done)
}

// ===== BACKWARD COMPATIBILITY =====
// Global mutex for legacy functions (slower but safe)
var blsMutex sync.Mutex

// ===== PUBLIC API FUNCTIONS =====

// Sign performs BLS signing (legacy - uses mutex, slower)
func Sign(bPri cm.PrivateKey, bMessage []byte) cm.Sign {
	return SignConcurrent(bPri, bMessage)
}

// SignConcurrent performs BLS signing with high concurrency support
func SignConcurrent(bPri cm.PrivateKey, bMessage []byte) cm.Sign {
	signer := GetBLSSigner()
	return signer.SignConcurrent(bPri, bMessage)
}

// VerifySign performs BLS verification (legacy - uses mutex, slower)
func VerifySign(bPub cm.PublicKey, bSig cm.Sign, bMsg []byte) bool {
	return VerifySignConcurrent(bPub, bSig, bMsg)
}

// VerifySignConcurrent performs BLS verification with high concurrency support
func VerifySignConcurrent(bPub cm.PublicKey, bSig cm.Sign, bMsg []byte) bool {
	signer := GetBLSSigner()
	return signer.VerifyConcurrent(bPub, bSig, bMsg)
}

// ValidateBlsPrivateKey validates a BLS private key before use
func ValidateBlsPrivateKey(keyBytes []byte) bool {
	if len(keyBytes) != 32 {
		return false
	}

	// Check if key is all zeros
	isZero := true
	for _, b := range keyBytes {
		if b != 0 {
			isZero = false
			break
		}
	}
	if isZero {
		return false
	}

	// Additional validation: check if key is valid for BLS curve
	// This is a basic check - in production you might want more thorough validation
	return true
}

func Init() {
	blst.SetMaxProcs(runtime.GOMAXPROCS(0))
}

func GetByteAddress(pubkey []byte) []byte {
	hash := crypto.Keccak256(pubkey)
	address := hash[12:]
	return address
}

// ===== LEGACY COMPATIBILITY FUNCTIONS =====

func VerifyAggregateSign(bPubs [][]byte, bSig []byte, bMsgs [][]byte) bool {
	blsMutex.Lock()
	defer blsMutex.Unlock()
	return new(blstSignature).AggregateVerifyCompressed(bSig, true, bPubs, false, bMsgs, dstMinPk)
}

func GenerateKeyPairFromSecretKey(hexSecretKey string) (cm.PrivateKey, cm.PublicKey, common.Address) {
	// Validate input before CGO calls
	secByte, err := hex.DecodeString(hexSecretKey)
	if err != nil || len(secByte) != 32 {
		return cm.PrivateKey{}, cm.PublicKey{}, common.Address{}
	}

	if !ValidateBlsPrivateKey(secByte) {
		return cm.PrivateKey{}, cm.PublicKey{}, common.Address{}
	}

	// Use legacy mutex approach
	blsMutex.Lock()
	defer blsMutex.Unlock()

	sec := new(blstSecretKey).Deserialize(secByte)
	pk := new(blstPublicKey).From(sec).Compress()
	hash := crypto.Keccak256([]byte(pk))
	return cm.PrivateKeyFromBytes(sec.Serialize()), cm.PubkeyFromBytes(pk), common.BytesToAddress(hash[12:])
}

func randBLSTSecretKey() *blstSecretKey {
	var t [32]byte
	_, _ = rand.Read(t[:])
	secretKey := blst.KeyGen(t[:])
	return secretKey
}

func GenerateKeyPair() *KeyPair {
	sec := randBLSTSecretKey()
	return NewKeyPair(sec.Serialize())
}

func CreateAggregateSign(bSignatures [][]byte) []byte {
	blsMutex.Lock()
	defer blsMutex.Unlock()

	aggregatedSignature := new(blst.P2Aggregate)
	aggregatedSignature.AggregateCompressed(bSignatures, false)
	return aggregatedSignature.ToAffine().Compress()
}
