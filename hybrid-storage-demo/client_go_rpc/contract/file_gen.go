// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// DownloadSession is an auto generated low-level Go binding around an user-defined struct.
type DownloadSession struct {
	FileKey       [32]byte
	User          common.Address
	Confirmations []common.Address
	IsConfirmed   bool
}

// FileProgress is an auto generated low-level Go binding around an user-defined struct.
type FileProgress struct {
	LastChunkHash   [32]byte
	ProcessedChunks uint64
	ProcessedLength *big.Int
}

// Info is an auto generated low-level Go binding around an user-defined struct.
type Info struct {
	Owner              common.Address
	MerkleRoot         [32]byte
	ContentLen         uint64
	TotalChunks        uint64
	ExpireTime         uint64
	Name               string
	Ext                string
	ContentDisposition string
	ContentID          string
	Status             uint8
}

// FileContractMetaData contains all meta data concerning the FileContract contract.
var FileContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"payable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"chunkIndex\",\"type\":\"uint256\"}],\"name\":\"ChunkUploaded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"downloadKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"DownloadKeyConfirmed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"downloadKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"DownloadKeyGenerated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"FileActivated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"contentLen\",\"type\":\"uint64\"}],\"name\":\"FileAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"FileDeleted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"FileLocked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"FundsWithdrawn\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"payer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"downloadCount\",\"type\":\"uint256\"}],\"name\":\"PaymentReceived\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"downloadKey\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"storageServer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"currentConfirmations\",\"type\":\"uint256\"}],\"name\":\"StorageConfirmed\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"addOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_server\",\"type\":\"address\"}],\"name\":\"addStorageServer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_validator\",\"type\":\"address\"}],\"name\":\"addValidator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"numChunks\",\"type\":\"uint256\"}],\"name\":\"calculatePrice\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"confirmFileActive\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"downloadKey\",\"type\":\"bytes32\"}],\"name\":\"confirmServerDownload\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"deleteFile\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"start\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"limit\",\"type\":\"uint256\"}],\"name\":\"downloadFile\",\"outputs\":[{\"internalType\":\"bytes[]\",\"name\":\"\",\"type\":\"bytes[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getContractBalance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"downloadKey\",\"type\":\"bytes32\"}],\"name\":\"getDownloadSessionInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"address[]\",\"name\":\"confirmations\",\"type\":\"address[]\"},{\"internalType\":\"bool\",\"name\":\"isConfirmed\",\"type\":\"bool\"}],\"internalType\":\"structDownloadSession\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"getFileInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"merkleRoot\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"contentLen\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"totalChunks\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"expireTime\",\"type\":\"uint64\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ext\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentDisposition\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentID\",\"type\":\"string\"},{\"internalType\":\"enumFileStatus\",\"name\":\"status\",\"type\":\"uint8\"}],\"internalType\":\"structInfo\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string[]\",\"name\":\"names\",\"type\":\"string[]\"}],\"name\":\"getFileKeyFromName\",\"outputs\":[{\"internalType\":\"bytes32[]\",\"name\":\"\",\"type\":\"bytes32[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"getFileProgress\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"lastChunkHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"processedChunks\",\"type\":\"uint64\"},{\"internalType\":\"uint256\",\"name\":\"processedLength\",\"type\":\"uint256\"}],\"internalType\":\"structFileProgress\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32[]\",\"name\":\"fileKeys\",\"type\":\"bytes32[]\"}],\"name\":\"getFilesInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"merkleRoot\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"contentLen\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"totalChunks\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"expireTime\",\"type\":\"uint64\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ext\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentDisposition\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentID\",\"type\":\"string\"},{\"internalType\":\"enumFileStatus\",\"name\":\"status\",\"type\":\"uint8\"}],\"internalType\":\"structInfo[]\",\"name\":\"infos\",\"type\":\"tuple[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getOwnerList\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getStorageServerList\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getValidatorList\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isOwner\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isStorageServer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isValidator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"name\":\"lockFile\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"mDownloadKeyToSession\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"internalType\":\"address\",\"name\":\"user\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"isConfirmed\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"mKeyToFileInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"merkleRoot\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"contentLen\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"totalChunks\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"expireTime\",\"type\":\"uint64\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ext\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentDisposition\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentID\",\"type\":\"string\"},{\"internalType\":\"enumFileStatus\",\"name\":\"status\",\"type\":\"uint8\"}],\"internalType\":\"structInfo\",\"name\":\"info\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"lastChunkHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"processedChunks\",\"type\":\"uint64\"},{\"internalType\":\"uint256\",\"name\":\"processedLength\",\"type\":\"uint256\"}],\"internalType\":\"structFileProgress\",\"name\":\"progress\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"name\":\"mNameToFileKey\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"ownerList\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"owners\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"downloadTimes\",\"type\":\"uint256\"}],\"name\":\"payForDownload\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pricePerChunk\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"merkleRoot\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"contentLen\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"totalChunks\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"expireTime\",\"type\":\"uint64\"},{\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"ext\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentDisposition\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"contentID\",\"type\":\"string\"},{\"internalType\":\"enumFileStatus\",\"name\":\"status\",\"type\":\"uint8\"}],\"internalType\":\"structInfo\",\"name\":\"info\",\"type\":\"tuple\"}],\"name\":\"pushFileInfo\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"}],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"removeOwner\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_server\",\"type\":\"address\"}],\"name\":\"removeStorageServer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_validator\",\"type\":\"address\"}],\"name\":\"removeValidator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"internalType\":\"uint64\",\"name\":\"_newExpireTime\",\"type\":\"uint64\"}],\"name\":\"renewTime\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"service\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_newPrice\",\"type\":\"uint256\"}],\"name\":\"setPricePerChunk\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"storageServerList\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"storageServers\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fileKey\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"chunkData\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"chunkIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32[]\",\"name\":\"merkleProof\",\"type\":\"bytes32[]\"}],\"name\":\"uploadChunk\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"validatorList\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"validators\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"withdrawAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"withdrawFunds\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// FileContractABI is the input ABI used to generate the binding from.
// Deprecated: Use FileContractMetaData.ABI instead.
var FileContractABI = FileContractMetaData.ABI

// FileContract is an auto generated Go binding around an Ethereum contract.
type FileContract struct {
	FileContractCaller     // Read-only binding to the contract
	FileContractTransactor // Write-only binding to the contract
	FileContractFilterer   // Log filterer for contract events
}

// FileContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type FileContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FileContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FileContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FileContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FileContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FileContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FileContractSession struct {
	Contract     *FileContract     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// FileContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FileContractCallerSession struct {
	Contract *FileContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// FileContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FileContractTransactorSession struct {
	Contract     *FileContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// FileContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type FileContractRaw struct {
	Contract *FileContract // Generic contract binding to access the raw methods on
}

// FileContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FileContractCallerRaw struct {
	Contract *FileContractCaller // Generic read-only contract binding to access the raw methods on
}

// FileContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FileContractTransactorRaw struct {
	Contract *FileContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFileContract creates a new instance of FileContract, bound to a specific deployed contract.
func NewFileContract(address common.Address, backend bind.ContractBackend) (*FileContract, error) {
	contract, err := bindFileContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FileContract{FileContractCaller: FileContractCaller{contract: contract}, FileContractTransactor: FileContractTransactor{contract: contract}, FileContractFilterer: FileContractFilterer{contract: contract}}, nil
}

// NewFileContractCaller creates a new read-only instance of FileContract, bound to a specific deployed contract.
func NewFileContractCaller(address common.Address, caller bind.ContractCaller) (*FileContractCaller, error) {
	contract, err := bindFileContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FileContractCaller{contract: contract}, nil
}

// NewFileContractTransactor creates a new write-only instance of FileContract, bound to a specific deployed contract.
func NewFileContractTransactor(address common.Address, transactor bind.ContractTransactor) (*FileContractTransactor, error) {
	contract, err := bindFileContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FileContractTransactor{contract: contract}, nil
}

// NewFileContractFilterer creates a new log filterer instance of FileContract, bound to a specific deployed contract.
func NewFileContractFilterer(address common.Address, filterer bind.ContractFilterer) (*FileContractFilterer, error) {
	contract, err := bindFileContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FileContractFilterer{contract: contract}, nil
}

// bindFileContract binds a generic wrapper to an already deployed contract.
func bindFileContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := FileContractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FileContract *FileContractRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FileContract.Contract.FileContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FileContract *FileContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FileContract.Contract.FileContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FileContract *FileContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FileContract.Contract.FileContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FileContract *FileContractCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FileContract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FileContract *FileContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FileContract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FileContract *FileContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FileContract.Contract.contract.Transact(opts, method, params...)
}

// CalculatePrice is a free data retrieval call binding the contract method 0xae104265.
//
// Solidity: function calculatePrice(uint256 numChunks) view returns(uint256)
func (_FileContract *FileContractCaller) CalculatePrice(opts *bind.CallOpts, numChunks *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "calculatePrice", numChunks)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// CalculatePrice is a free data retrieval call binding the contract method 0xae104265.
//
// Solidity: function calculatePrice(uint256 numChunks) view returns(uint256)
func (_FileContract *FileContractSession) CalculatePrice(numChunks *big.Int) (*big.Int, error) {
	return _FileContract.Contract.CalculatePrice(&_FileContract.CallOpts, numChunks)
}

// CalculatePrice is a free data retrieval call binding the contract method 0xae104265.
//
// Solidity: function calculatePrice(uint256 numChunks) view returns(uint256)
func (_FileContract *FileContractCallerSession) CalculatePrice(numChunks *big.Int) (*big.Int, error) {
	return _FileContract.Contract.CalculatePrice(&_FileContract.CallOpts, numChunks)
}

// DownloadFile is a free data retrieval call binding the contract method 0xfab91940.
//
// Solidity: function downloadFile(bytes32 fileKey, uint256 start, uint256 limit) view returns(bytes[])
func (_FileContract *FileContractCaller) DownloadFile(opts *bind.CallOpts, fileKey [32]byte, start *big.Int, limit *big.Int) ([][]byte, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "downloadFile", fileKey, start, limit)

	if err != nil {
		return *new([][]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([][]byte)).(*[][]byte)

	return out0, err

}

// DownloadFile is a free data retrieval call binding the contract method 0xfab91940.
//
// Solidity: function downloadFile(bytes32 fileKey, uint256 start, uint256 limit) view returns(bytes[])
func (_FileContract *FileContractSession) DownloadFile(fileKey [32]byte, start *big.Int, limit *big.Int) ([][]byte, error) {
	return _FileContract.Contract.DownloadFile(&_FileContract.CallOpts, fileKey, start, limit)
}

// DownloadFile is a free data retrieval call binding the contract method 0xfab91940.
//
// Solidity: function downloadFile(bytes32 fileKey, uint256 start, uint256 limit) view returns(bytes[])
func (_FileContract *FileContractCallerSession) DownloadFile(fileKey [32]byte, start *big.Int, limit *big.Int) ([][]byte, error) {
	return _FileContract.Contract.DownloadFile(&_FileContract.CallOpts, fileKey, start, limit)
}

// GetContractBalance is a free data retrieval call binding the contract method 0x6f9fb98a.
//
// Solidity: function getContractBalance() view returns(uint256)
func (_FileContract *FileContractCaller) GetContractBalance(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getContractBalance")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetContractBalance is a free data retrieval call binding the contract method 0x6f9fb98a.
//
// Solidity: function getContractBalance() view returns(uint256)
func (_FileContract *FileContractSession) GetContractBalance() (*big.Int, error) {
	return _FileContract.Contract.GetContractBalance(&_FileContract.CallOpts)
}

// GetContractBalance is a free data retrieval call binding the contract method 0x6f9fb98a.
//
// Solidity: function getContractBalance() view returns(uint256)
func (_FileContract *FileContractCallerSession) GetContractBalance() (*big.Int, error) {
	return _FileContract.Contract.GetContractBalance(&_FileContract.CallOpts)
}

// GetDownloadSessionInfo is a free data retrieval call binding the contract method 0x61d8e863.
//
// Solidity: function getDownloadSessionInfo(bytes32 downloadKey) view returns((bytes32,address,address[],bool))
func (_FileContract *FileContractCaller) GetDownloadSessionInfo(opts *bind.CallOpts, downloadKey [32]byte) (DownloadSession, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getDownloadSessionInfo", downloadKey)

	if err != nil {
		return *new(DownloadSession), err
	}

	out0 := *abi.ConvertType(out[0], new(DownloadSession)).(*DownloadSession)

	return out0, err

}

// GetDownloadSessionInfo is a free data retrieval call binding the contract method 0x61d8e863.
//
// Solidity: function getDownloadSessionInfo(bytes32 downloadKey) view returns((bytes32,address,address[],bool))
func (_FileContract *FileContractSession) GetDownloadSessionInfo(downloadKey [32]byte) (DownloadSession, error) {
	return _FileContract.Contract.GetDownloadSessionInfo(&_FileContract.CallOpts, downloadKey)
}

// GetDownloadSessionInfo is a free data retrieval call binding the contract method 0x61d8e863.
//
// Solidity: function getDownloadSessionInfo(bytes32 downloadKey) view returns((bytes32,address,address[],bool))
func (_FileContract *FileContractCallerSession) GetDownloadSessionInfo(downloadKey [32]byte) (DownloadSession, error) {
	return _FileContract.Contract.GetDownloadSessionInfo(&_FileContract.CallOpts, downloadKey)
}

// GetFileInfo is a free data retrieval call binding the contract method 0x52f7347b.
//
// Solidity: function getFileInfo(bytes32 fileKey) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8))
func (_FileContract *FileContractCaller) GetFileInfo(opts *bind.CallOpts, fileKey [32]byte) (Info, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getFileInfo", fileKey)

	if err != nil {
		return *new(Info), err
	}

	out0 := *abi.ConvertType(out[0], new(Info)).(*Info)

	return out0, err

}

// GetFileInfo is a free data retrieval call binding the contract method 0x52f7347b.
//
// Solidity: function getFileInfo(bytes32 fileKey) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8))
func (_FileContract *FileContractSession) GetFileInfo(fileKey [32]byte) (Info, error) {
	return _FileContract.Contract.GetFileInfo(&_FileContract.CallOpts, fileKey)
}

// GetFileInfo is a free data retrieval call binding the contract method 0x52f7347b.
//
// Solidity: function getFileInfo(bytes32 fileKey) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8))
func (_FileContract *FileContractCallerSession) GetFileInfo(fileKey [32]byte) (Info, error) {
	return _FileContract.Contract.GetFileInfo(&_FileContract.CallOpts, fileKey)
}

// GetFileKeyFromName is a free data retrieval call binding the contract method 0xf25671c4.
//
// Solidity: function getFileKeyFromName(string[] names) view returns(bytes32[])
func (_FileContract *FileContractCaller) GetFileKeyFromName(opts *bind.CallOpts, names []string) ([][32]byte, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getFileKeyFromName", names)

	if err != nil {
		return *new([][32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([][32]byte)).(*[][32]byte)

	return out0, err

}

// GetFileKeyFromName is a free data retrieval call binding the contract method 0xf25671c4.
//
// Solidity: function getFileKeyFromName(string[] names) view returns(bytes32[])
func (_FileContract *FileContractSession) GetFileKeyFromName(names []string) ([][32]byte, error) {
	return _FileContract.Contract.GetFileKeyFromName(&_FileContract.CallOpts, names)
}

// GetFileKeyFromName is a free data retrieval call binding the contract method 0xf25671c4.
//
// Solidity: function getFileKeyFromName(string[] names) view returns(bytes32[])
func (_FileContract *FileContractCallerSession) GetFileKeyFromName(names []string) ([][32]byte, error) {
	return _FileContract.Contract.GetFileKeyFromName(&_FileContract.CallOpts, names)
}

// GetFileProgress is a free data retrieval call binding the contract method 0x46da5178.
//
// Solidity: function getFileProgress(bytes32 fileKey) view returns((bytes32,uint64,uint256))
func (_FileContract *FileContractCaller) GetFileProgress(opts *bind.CallOpts, fileKey [32]byte) (FileProgress, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getFileProgress", fileKey)

	if err != nil {
		return *new(FileProgress), err
	}

	out0 := *abi.ConvertType(out[0], new(FileProgress)).(*FileProgress)

	return out0, err

}

// GetFileProgress is a free data retrieval call binding the contract method 0x46da5178.
//
// Solidity: function getFileProgress(bytes32 fileKey) view returns((bytes32,uint64,uint256))
func (_FileContract *FileContractSession) GetFileProgress(fileKey [32]byte) (FileProgress, error) {
	return _FileContract.Contract.GetFileProgress(&_FileContract.CallOpts, fileKey)
}

// GetFileProgress is a free data retrieval call binding the contract method 0x46da5178.
//
// Solidity: function getFileProgress(bytes32 fileKey) view returns((bytes32,uint64,uint256))
func (_FileContract *FileContractCallerSession) GetFileProgress(fileKey [32]byte) (FileProgress, error) {
	return _FileContract.Contract.GetFileProgress(&_FileContract.CallOpts, fileKey)
}

// GetFilesInfo is a free data retrieval call binding the contract method 0xeb070e7e.
//
// Solidity: function getFilesInfo(bytes32[] fileKeys) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8)[] infos)
func (_FileContract *FileContractCaller) GetFilesInfo(opts *bind.CallOpts, fileKeys [][32]byte) ([]Info, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getFilesInfo", fileKeys)

	if err != nil {
		return *new([]Info), err
	}

	out0 := *abi.ConvertType(out[0], new([]Info)).(*[]Info)

	return out0, err

}

// GetFilesInfo is a free data retrieval call binding the contract method 0xeb070e7e.
//
// Solidity: function getFilesInfo(bytes32[] fileKeys) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8)[] infos)
func (_FileContract *FileContractSession) GetFilesInfo(fileKeys [][32]byte) ([]Info, error) {
	return _FileContract.Contract.GetFilesInfo(&_FileContract.CallOpts, fileKeys)
}

// GetFilesInfo is a free data retrieval call binding the contract method 0xeb070e7e.
//
// Solidity: function getFilesInfo(bytes32[] fileKeys) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8)[] infos)
func (_FileContract *FileContractCallerSession) GetFilesInfo(fileKeys [][32]byte) ([]Info, error) {
	return _FileContract.Contract.GetFilesInfo(&_FileContract.CallOpts, fileKeys)
}

// GetOwnerList is a free data retrieval call binding the contract method 0x58100370.
//
// Solidity: function getOwnerList() view returns(address[])
func (_FileContract *FileContractCaller) GetOwnerList(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getOwnerList")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetOwnerList is a free data retrieval call binding the contract method 0x58100370.
//
// Solidity: function getOwnerList() view returns(address[])
func (_FileContract *FileContractSession) GetOwnerList() ([]common.Address, error) {
	return _FileContract.Contract.GetOwnerList(&_FileContract.CallOpts)
}

// GetOwnerList is a free data retrieval call binding the contract method 0x58100370.
//
// Solidity: function getOwnerList() view returns(address[])
func (_FileContract *FileContractCallerSession) GetOwnerList() ([]common.Address, error) {
	return _FileContract.Contract.GetOwnerList(&_FileContract.CallOpts)
}

// GetStorageServerList is a free data retrieval call binding the contract method 0x992b0106.
//
// Solidity: function getStorageServerList() view returns(address[])
func (_FileContract *FileContractCaller) GetStorageServerList(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getStorageServerList")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetStorageServerList is a free data retrieval call binding the contract method 0x992b0106.
//
// Solidity: function getStorageServerList() view returns(address[])
func (_FileContract *FileContractSession) GetStorageServerList() ([]common.Address, error) {
	return _FileContract.Contract.GetStorageServerList(&_FileContract.CallOpts)
}

// GetStorageServerList is a free data retrieval call binding the contract method 0x992b0106.
//
// Solidity: function getStorageServerList() view returns(address[])
func (_FileContract *FileContractCallerSession) GetStorageServerList() ([]common.Address, error) {
	return _FileContract.Contract.GetStorageServerList(&_FileContract.CallOpts)
}

// GetValidatorList is a free data retrieval call binding the contract method 0xe35c0f7d.
//
// Solidity: function getValidatorList() view returns(address[])
func (_FileContract *FileContractCaller) GetValidatorList(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "getValidatorList")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetValidatorList is a free data retrieval call binding the contract method 0xe35c0f7d.
//
// Solidity: function getValidatorList() view returns(address[])
func (_FileContract *FileContractSession) GetValidatorList() ([]common.Address, error) {
	return _FileContract.Contract.GetValidatorList(&_FileContract.CallOpts)
}

// GetValidatorList is a free data retrieval call binding the contract method 0xe35c0f7d.
//
// Solidity: function getValidatorList() view returns(address[])
func (_FileContract *FileContractCallerSession) GetValidatorList() ([]common.Address, error) {
	return _FileContract.Contract.GetValidatorList(&_FileContract.CallOpts)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) view returns(bool)
func (_FileContract *FileContractCaller) IsOwner(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "isOwner", _address)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) view returns(bool)
func (_FileContract *FileContractSession) IsOwner(_address common.Address) (bool, error) {
	return _FileContract.Contract.IsOwner(&_FileContract.CallOpts, _address)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(address _address) view returns(bool)
func (_FileContract *FileContractCallerSession) IsOwner(_address common.Address) (bool, error) {
	return _FileContract.Contract.IsOwner(&_FileContract.CallOpts, _address)
}

// IsStorageServer is a free data retrieval call binding the contract method 0x8939210c.
//
// Solidity: function isStorageServer(address _address) view returns(bool)
func (_FileContract *FileContractCaller) IsStorageServer(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "isStorageServer", _address)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsStorageServer is a free data retrieval call binding the contract method 0x8939210c.
//
// Solidity: function isStorageServer(address _address) view returns(bool)
func (_FileContract *FileContractSession) IsStorageServer(_address common.Address) (bool, error) {
	return _FileContract.Contract.IsStorageServer(&_FileContract.CallOpts, _address)
}

// IsStorageServer is a free data retrieval call binding the contract method 0x8939210c.
//
// Solidity: function isStorageServer(address _address) view returns(bool)
func (_FileContract *FileContractCallerSession) IsStorageServer(_address common.Address) (bool, error) {
	return _FileContract.Contract.IsStorageServer(&_FileContract.CallOpts, _address)
}

// IsValidator is a free data retrieval call binding the contract method 0xfacd743b.
//
// Solidity: function isValidator(address _address) view returns(bool)
func (_FileContract *FileContractCaller) IsValidator(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "isValidator", _address)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidator is a free data retrieval call binding the contract method 0xfacd743b.
//
// Solidity: function isValidator(address _address) view returns(bool)
func (_FileContract *FileContractSession) IsValidator(_address common.Address) (bool, error) {
	return _FileContract.Contract.IsValidator(&_FileContract.CallOpts, _address)
}

// IsValidator is a free data retrieval call binding the contract method 0xfacd743b.
//
// Solidity: function isValidator(address _address) view returns(bool)
func (_FileContract *FileContractCallerSession) IsValidator(_address common.Address) (bool, error) {
	return _FileContract.Contract.IsValidator(&_FileContract.CallOpts, _address)
}

// MDownloadKeyToSession is a free data retrieval call binding the contract method 0xa19d4b19.
//
// Solidity: function mDownloadKeyToSession(bytes32 ) view returns(bytes32 fileKey, address user, bool isConfirmed)
func (_FileContract *FileContractCaller) MDownloadKeyToSession(opts *bind.CallOpts, arg0 [32]byte) (struct {
	FileKey     [32]byte
	User        common.Address
	IsConfirmed bool
}, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "mDownloadKeyToSession", arg0)

	outstruct := new(struct {
		FileKey     [32]byte
		User        common.Address
		IsConfirmed bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.FileKey = *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)
	outstruct.User = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.IsConfirmed = *abi.ConvertType(out[2], new(bool)).(*bool)

	return *outstruct, err

}

// MDownloadKeyToSession is a free data retrieval call binding the contract method 0xa19d4b19.
//
// Solidity: function mDownloadKeyToSession(bytes32 ) view returns(bytes32 fileKey, address user, bool isConfirmed)
func (_FileContract *FileContractSession) MDownloadKeyToSession(arg0 [32]byte) (struct {
	FileKey     [32]byte
	User        common.Address
	IsConfirmed bool
}, error) {
	return _FileContract.Contract.MDownloadKeyToSession(&_FileContract.CallOpts, arg0)
}

// MDownloadKeyToSession is a free data retrieval call binding the contract method 0xa19d4b19.
//
// Solidity: function mDownloadKeyToSession(bytes32 ) view returns(bytes32 fileKey, address user, bool isConfirmed)
func (_FileContract *FileContractCallerSession) MDownloadKeyToSession(arg0 [32]byte) (struct {
	FileKey     [32]byte
	User        common.Address
	IsConfirmed bool
}, error) {
	return _FileContract.Contract.MDownloadKeyToSession(&_FileContract.CallOpts, arg0)
}

// MKeyToFileInfo is a free data retrieval call binding the contract method 0xc652793d.
//
// Solidity: function mKeyToFileInfo(bytes32 ) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8) info, (bytes32,uint64,uint256) progress)
func (_FileContract *FileContractCaller) MKeyToFileInfo(opts *bind.CallOpts, arg0 [32]byte) (struct {
	Info     Info
	Progress FileProgress
}, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "mKeyToFileInfo", arg0)

	outstruct := new(struct {
		Info     Info
		Progress FileProgress
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Info = *abi.ConvertType(out[0], new(Info)).(*Info)
	outstruct.Progress = *abi.ConvertType(out[1], new(FileProgress)).(*FileProgress)

	return *outstruct, err

}

// MKeyToFileInfo is a free data retrieval call binding the contract method 0xc652793d.
//
// Solidity: function mKeyToFileInfo(bytes32 ) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8) info, (bytes32,uint64,uint256) progress)
func (_FileContract *FileContractSession) MKeyToFileInfo(arg0 [32]byte) (struct {
	Info     Info
	Progress FileProgress
}, error) {
	return _FileContract.Contract.MKeyToFileInfo(&_FileContract.CallOpts, arg0)
}

// MKeyToFileInfo is a free data retrieval call binding the contract method 0xc652793d.
//
// Solidity: function mKeyToFileInfo(bytes32 ) view returns((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8) info, (bytes32,uint64,uint256) progress)
func (_FileContract *FileContractCallerSession) MKeyToFileInfo(arg0 [32]byte) (struct {
	Info     Info
	Progress FileProgress
}, error) {
	return _FileContract.Contract.MKeyToFileInfo(&_FileContract.CallOpts, arg0)
}

// MNameToFileKey is a free data retrieval call binding the contract method 0xedff13af.
//
// Solidity: function mNameToFileKey(string ) view returns(bytes32)
func (_FileContract *FileContractCaller) MNameToFileKey(opts *bind.CallOpts, arg0 string) ([32]byte, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "mNameToFileKey", arg0)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// MNameToFileKey is a free data retrieval call binding the contract method 0xedff13af.
//
// Solidity: function mNameToFileKey(string ) view returns(bytes32)
func (_FileContract *FileContractSession) MNameToFileKey(arg0 string) ([32]byte, error) {
	return _FileContract.Contract.MNameToFileKey(&_FileContract.CallOpts, arg0)
}

// MNameToFileKey is a free data retrieval call binding the contract method 0xedff13af.
//
// Solidity: function mNameToFileKey(string ) view returns(bytes32)
func (_FileContract *FileContractCallerSession) MNameToFileKey(arg0 string) ([32]byte, error) {
	return _FileContract.Contract.MNameToFileKey(&_FileContract.CallOpts, arg0)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FileContract *FileContractCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FileContract *FileContractSession) Owner() (common.Address, error) {
	return _FileContract.Contract.Owner(&_FileContract.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_FileContract *FileContractCallerSession) Owner() (common.Address, error) {
	return _FileContract.Contract.Owner(&_FileContract.CallOpts)
}

// OwnerList is a free data retrieval call binding the contract method 0xdef79ab5.
//
// Solidity: function ownerList(uint256 ) view returns(address)
func (_FileContract *FileContractCaller) OwnerList(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "ownerList", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// OwnerList is a free data retrieval call binding the contract method 0xdef79ab5.
//
// Solidity: function ownerList(uint256 ) view returns(address)
func (_FileContract *FileContractSession) OwnerList(arg0 *big.Int) (common.Address, error) {
	return _FileContract.Contract.OwnerList(&_FileContract.CallOpts, arg0)
}

// OwnerList is a free data retrieval call binding the contract method 0xdef79ab5.
//
// Solidity: function ownerList(uint256 ) view returns(address)
func (_FileContract *FileContractCallerSession) OwnerList(arg0 *big.Int) (common.Address, error) {
	return _FileContract.Contract.OwnerList(&_FileContract.CallOpts, arg0)
}

// Owners is a free data retrieval call binding the contract method 0x022914a7.
//
// Solidity: function owners(address ) view returns(bool)
func (_FileContract *FileContractCaller) Owners(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "owners", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Owners is a free data retrieval call binding the contract method 0x022914a7.
//
// Solidity: function owners(address ) view returns(bool)
func (_FileContract *FileContractSession) Owners(arg0 common.Address) (bool, error) {
	return _FileContract.Contract.Owners(&_FileContract.CallOpts, arg0)
}

// Owners is a free data retrieval call binding the contract method 0x022914a7.
//
// Solidity: function owners(address ) view returns(bool)
func (_FileContract *FileContractCallerSession) Owners(arg0 common.Address) (bool, error) {
	return _FileContract.Contract.Owners(&_FileContract.CallOpts, arg0)
}

// PricePerChunk is a free data retrieval call binding the contract method 0xcb27f370.
//
// Solidity: function pricePerChunk() view returns(uint256)
func (_FileContract *FileContractCaller) PricePerChunk(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "pricePerChunk")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PricePerChunk is a free data retrieval call binding the contract method 0xcb27f370.
//
// Solidity: function pricePerChunk() view returns(uint256)
func (_FileContract *FileContractSession) PricePerChunk() (*big.Int, error) {
	return _FileContract.Contract.PricePerChunk(&_FileContract.CallOpts)
}

// PricePerChunk is a free data retrieval call binding the contract method 0xcb27f370.
//
// Solidity: function pricePerChunk() view returns(uint256)
func (_FileContract *FileContractCallerSession) PricePerChunk() (*big.Int, error) {
	return _FileContract.Contract.PricePerChunk(&_FileContract.CallOpts)
}

// Service is a free data retrieval call binding the contract method 0xd598d4c9.
//
// Solidity: function service() view returns(address)
func (_FileContract *FileContractCaller) Service(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "service")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Service is a free data retrieval call binding the contract method 0xd598d4c9.
//
// Solidity: function service() view returns(address)
func (_FileContract *FileContractSession) Service() (common.Address, error) {
	return _FileContract.Contract.Service(&_FileContract.CallOpts)
}

// Service is a free data retrieval call binding the contract method 0xd598d4c9.
//
// Solidity: function service() view returns(address)
func (_FileContract *FileContractCallerSession) Service() (common.Address, error) {
	return _FileContract.Contract.Service(&_FileContract.CallOpts)
}

// StorageServerList is a free data retrieval call binding the contract method 0x5d420eaf.
//
// Solidity: function storageServerList(uint256 ) view returns(address)
func (_FileContract *FileContractCaller) StorageServerList(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "storageServerList", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StorageServerList is a free data retrieval call binding the contract method 0x5d420eaf.
//
// Solidity: function storageServerList(uint256 ) view returns(address)
func (_FileContract *FileContractSession) StorageServerList(arg0 *big.Int) (common.Address, error) {
	return _FileContract.Contract.StorageServerList(&_FileContract.CallOpts, arg0)
}

// StorageServerList is a free data retrieval call binding the contract method 0x5d420eaf.
//
// Solidity: function storageServerList(uint256 ) view returns(address)
func (_FileContract *FileContractCallerSession) StorageServerList(arg0 *big.Int) (common.Address, error) {
	return _FileContract.Contract.StorageServerList(&_FileContract.CallOpts, arg0)
}

// StorageServers is a free data retrieval call binding the contract method 0xd51cbc4d.
//
// Solidity: function storageServers(address ) view returns(bool)
func (_FileContract *FileContractCaller) StorageServers(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "storageServers", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// StorageServers is a free data retrieval call binding the contract method 0xd51cbc4d.
//
// Solidity: function storageServers(address ) view returns(bool)
func (_FileContract *FileContractSession) StorageServers(arg0 common.Address) (bool, error) {
	return _FileContract.Contract.StorageServers(&_FileContract.CallOpts, arg0)
}

// StorageServers is a free data retrieval call binding the contract method 0xd51cbc4d.
//
// Solidity: function storageServers(address ) view returns(bool)
func (_FileContract *FileContractCallerSession) StorageServers(arg0 common.Address) (bool, error) {
	return _FileContract.Contract.StorageServers(&_FileContract.CallOpts, arg0)
}

// ValidatorList is a free data retrieval call binding the contract method 0xb048e056.
//
// Solidity: function validatorList(uint256 ) view returns(address)
func (_FileContract *FileContractCaller) ValidatorList(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "validatorList", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ValidatorList is a free data retrieval call binding the contract method 0xb048e056.
//
// Solidity: function validatorList(uint256 ) view returns(address)
func (_FileContract *FileContractSession) ValidatorList(arg0 *big.Int) (common.Address, error) {
	return _FileContract.Contract.ValidatorList(&_FileContract.CallOpts, arg0)
}

// ValidatorList is a free data retrieval call binding the contract method 0xb048e056.
//
// Solidity: function validatorList(uint256 ) view returns(address)
func (_FileContract *FileContractCallerSession) ValidatorList(arg0 *big.Int) (common.Address, error) {
	return _FileContract.Contract.ValidatorList(&_FileContract.CallOpts, arg0)
}

// Validators is a free data retrieval call binding the contract method 0xfa52c7d8.
//
// Solidity: function validators(address ) view returns(bool)
func (_FileContract *FileContractCaller) Validators(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "validators", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Validators is a free data retrieval call binding the contract method 0xfa52c7d8.
//
// Solidity: function validators(address ) view returns(bool)
func (_FileContract *FileContractSession) Validators(arg0 common.Address) (bool, error) {
	return _FileContract.Contract.Validators(&_FileContract.CallOpts, arg0)
}

// Validators is a free data retrieval call binding the contract method 0xfa52c7d8.
//
// Solidity: function validators(address ) view returns(bool)
func (_FileContract *FileContractCallerSession) Validators(arg0 common.Address) (bool, error) {
	return _FileContract.Contract.Validators(&_FileContract.CallOpts, arg0)
}

// AddOwner is a paid mutator transaction binding the contract method 0x7065cb48.
//
// Solidity: function addOwner(address _owner) returns()
func (_FileContract *FileContractTransactor) AddOwner(opts *bind.TransactOpts, _owner common.Address) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "addOwner", _owner)
}

// AddOwner is a paid mutator transaction binding the contract method 0x7065cb48.
//
// Solidity: function addOwner(address _owner) returns()
func (_FileContract *FileContractSession) AddOwner(_owner common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.AddOwner(&_FileContract.TransactOpts, _owner)
}

// AddOwner is a paid mutator transaction binding the contract method 0x7065cb48.
//
// Solidity: function addOwner(address _owner) returns()
func (_FileContract *FileContractTransactorSession) AddOwner(_owner common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.AddOwner(&_FileContract.TransactOpts, _owner)
}

// AddStorageServer is a paid mutator transaction binding the contract method 0x21777551.
//
// Solidity: function addStorageServer(address _server) returns()
func (_FileContract *FileContractTransactor) AddStorageServer(opts *bind.TransactOpts, _server common.Address) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "addStorageServer", _server)
}

// AddStorageServer is a paid mutator transaction binding the contract method 0x21777551.
//
// Solidity: function addStorageServer(address _server) returns()
func (_FileContract *FileContractSession) AddStorageServer(_server common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.AddStorageServer(&_FileContract.TransactOpts, _server)
}

// AddStorageServer is a paid mutator transaction binding the contract method 0x21777551.
//
// Solidity: function addStorageServer(address _server) returns()
func (_FileContract *FileContractTransactorSession) AddStorageServer(_server common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.AddStorageServer(&_FileContract.TransactOpts, _server)
}

// AddValidator is a paid mutator transaction binding the contract method 0x4d238c8e.
//
// Solidity: function addValidator(address _validator) returns()
func (_FileContract *FileContractTransactor) AddValidator(opts *bind.TransactOpts, _validator common.Address) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "addValidator", _validator)
}

// AddValidator is a paid mutator transaction binding the contract method 0x4d238c8e.
//
// Solidity: function addValidator(address _validator) returns()
func (_FileContract *FileContractSession) AddValidator(_validator common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.AddValidator(&_FileContract.TransactOpts, _validator)
}

// AddValidator is a paid mutator transaction binding the contract method 0x4d238c8e.
//
// Solidity: function addValidator(address _validator) returns()
func (_FileContract *FileContractTransactorSession) AddValidator(_validator common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.AddValidator(&_FileContract.TransactOpts, _validator)
}

// ConfirmFileActive is a paid mutator transaction binding the contract method 0x350190e5.
//
// Solidity: function confirmFileActive(bytes32 fileKey) returns()
func (_FileContract *FileContractTransactor) ConfirmFileActive(opts *bind.TransactOpts, fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "confirmFileActive", fileKey)
}

// ConfirmFileActive is a paid mutator transaction binding the contract method 0x350190e5.
//
// Solidity: function confirmFileActive(bytes32 fileKey) returns()
func (_FileContract *FileContractSession) ConfirmFileActive(fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.ConfirmFileActive(&_FileContract.TransactOpts, fileKey)
}

// ConfirmFileActive is a paid mutator transaction binding the contract method 0x350190e5.
//
// Solidity: function confirmFileActive(bytes32 fileKey) returns()
func (_FileContract *FileContractTransactorSession) ConfirmFileActive(fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.ConfirmFileActive(&_FileContract.TransactOpts, fileKey)
}

// ConfirmServerDownload is a paid mutator transaction binding the contract method 0xfb26faba.
//
// Solidity: function confirmServerDownload(bytes32 downloadKey) returns()
func (_FileContract *FileContractTransactor) ConfirmServerDownload(opts *bind.TransactOpts, downloadKey [32]byte) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "confirmServerDownload", downloadKey)
}

// ConfirmServerDownload is a paid mutator transaction binding the contract method 0xfb26faba.
//
// Solidity: function confirmServerDownload(bytes32 downloadKey) returns()
func (_FileContract *FileContractSession) ConfirmServerDownload(downloadKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.ConfirmServerDownload(&_FileContract.TransactOpts, downloadKey)
}

// ConfirmServerDownload is a paid mutator transaction binding the contract method 0xfb26faba.
//
// Solidity: function confirmServerDownload(bytes32 downloadKey) returns()
func (_FileContract *FileContractTransactorSession) ConfirmServerDownload(downloadKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.ConfirmServerDownload(&_FileContract.TransactOpts, downloadKey)
}

// DeleteFile is a paid mutator transaction binding the contract method 0x6ab799f1.
//
// Solidity: function deleteFile(bytes32 fileKey) returns()
func (_FileContract *FileContractTransactor) DeleteFile(opts *bind.TransactOpts, fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "deleteFile", fileKey)
}

// DeleteFile is a paid mutator transaction binding the contract method 0x6ab799f1.
//
// Solidity: function deleteFile(bytes32 fileKey) returns()
func (_FileContract *FileContractSession) DeleteFile(fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.DeleteFile(&_FileContract.TransactOpts, fileKey)
}

// DeleteFile is a paid mutator transaction binding the contract method 0x6ab799f1.
//
// Solidity: function deleteFile(bytes32 fileKey) returns()
func (_FileContract *FileContractTransactorSession) DeleteFile(fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.DeleteFile(&_FileContract.TransactOpts, fileKey)
}

// LockFile is a paid mutator transaction binding the contract method 0x9b26b2a8.
//
// Solidity: function lockFile(bytes32 fileKey) returns()
func (_FileContract *FileContractTransactor) LockFile(opts *bind.TransactOpts, fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "lockFile", fileKey)
}

// LockFile is a paid mutator transaction binding the contract method 0x9b26b2a8.
//
// Solidity: function lockFile(bytes32 fileKey) returns()
func (_FileContract *FileContractSession) LockFile(fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.LockFile(&_FileContract.TransactOpts, fileKey)
}

// LockFile is a paid mutator transaction binding the contract method 0x9b26b2a8.
//
// Solidity: function lockFile(bytes32 fileKey) returns()
func (_FileContract *FileContractTransactorSession) LockFile(fileKey [32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.LockFile(&_FileContract.TransactOpts, fileKey)
}

// PayForDownload is a paid mutator transaction binding the contract method 0xb88eb24b.
//
// Solidity: function payForDownload(bytes32 fileKey, uint256 downloadTimes) payable returns()
func (_FileContract *FileContractTransactor) PayForDownload(opts *bind.TransactOpts, fileKey [32]byte, downloadTimes *big.Int) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "payForDownload", fileKey, downloadTimes)
}

// PayForDownload is a paid mutator transaction binding the contract method 0xb88eb24b.
//
// Solidity: function payForDownload(bytes32 fileKey, uint256 downloadTimes) payable returns()
func (_FileContract *FileContractSession) PayForDownload(fileKey [32]byte, downloadTimes *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.PayForDownload(&_FileContract.TransactOpts, fileKey, downloadTimes)
}

// PayForDownload is a paid mutator transaction binding the contract method 0xb88eb24b.
//
// Solidity: function payForDownload(bytes32 fileKey, uint256 downloadTimes) payable returns()
func (_FileContract *FileContractTransactorSession) PayForDownload(fileKey [32]byte, downloadTimes *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.PayForDownload(&_FileContract.TransactOpts, fileKey, downloadTimes)
}

// PushFileInfo is a paid mutator transaction binding the contract method 0x20ca38ed.
//
// Solidity: function pushFileInfo((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8) info) payable returns(bytes32 fileKey)
func (_FileContract *FileContractTransactor) PushFileInfo(opts *bind.TransactOpts, info Info) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "pushFileInfo", info)
}

// PushFileInfo is a paid mutator transaction binding the contract method 0x20ca38ed.
//
// Solidity: function pushFileInfo((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8) info) payable returns(bytes32 fileKey)
func (_FileContract *FileContractSession) PushFileInfo(info Info) (*types.Transaction, error) {
	return _FileContract.Contract.PushFileInfo(&_FileContract.TransactOpts, info)
}

// PushFileInfo is a paid mutator transaction binding the contract method 0x20ca38ed.
//
// Solidity: function pushFileInfo((address,bytes32,uint64,uint64,uint64,string,string,string,string,uint8) info) payable returns(bytes32 fileKey)
func (_FileContract *FileContractTransactorSession) PushFileInfo(info Info) (*types.Transaction, error) {
	return _FileContract.Contract.PushFileInfo(&_FileContract.TransactOpts, info)
}

// RemoveOwner is a paid mutator transaction binding the contract method 0x173825d9.
//
// Solidity: function removeOwner(address _owner) returns()
func (_FileContract *FileContractTransactor) RemoveOwner(opts *bind.TransactOpts, _owner common.Address) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "removeOwner", _owner)
}

// RemoveOwner is a paid mutator transaction binding the contract method 0x173825d9.
//
// Solidity: function removeOwner(address _owner) returns()
func (_FileContract *FileContractSession) RemoveOwner(_owner common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.RemoveOwner(&_FileContract.TransactOpts, _owner)
}

// RemoveOwner is a paid mutator transaction binding the contract method 0x173825d9.
//
// Solidity: function removeOwner(address _owner) returns()
func (_FileContract *FileContractTransactorSession) RemoveOwner(_owner common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.RemoveOwner(&_FileContract.TransactOpts, _owner)
}

// RemoveStorageServer is a paid mutator transaction binding the contract method 0x61577fe6.
//
// Solidity: function removeStorageServer(address _server) returns()
func (_FileContract *FileContractTransactor) RemoveStorageServer(opts *bind.TransactOpts, _server common.Address) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "removeStorageServer", _server)
}

// RemoveStorageServer is a paid mutator transaction binding the contract method 0x61577fe6.
//
// Solidity: function removeStorageServer(address _server) returns()
func (_FileContract *FileContractSession) RemoveStorageServer(_server common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.RemoveStorageServer(&_FileContract.TransactOpts, _server)
}

// RemoveStorageServer is a paid mutator transaction binding the contract method 0x61577fe6.
//
// Solidity: function removeStorageServer(address _server) returns()
func (_FileContract *FileContractTransactorSession) RemoveStorageServer(_server common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.RemoveStorageServer(&_FileContract.TransactOpts, _server)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x40a141ff.
//
// Solidity: function removeValidator(address _validator) returns()
func (_FileContract *FileContractTransactor) RemoveValidator(opts *bind.TransactOpts, _validator common.Address) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "removeValidator", _validator)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x40a141ff.
//
// Solidity: function removeValidator(address _validator) returns()
func (_FileContract *FileContractSession) RemoveValidator(_validator common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.RemoveValidator(&_FileContract.TransactOpts, _validator)
}

// RemoveValidator is a paid mutator transaction binding the contract method 0x40a141ff.
//
// Solidity: function removeValidator(address _validator) returns()
func (_FileContract *FileContractTransactorSession) RemoveValidator(_validator common.Address) (*types.Transaction, error) {
	return _FileContract.Contract.RemoveValidator(&_FileContract.TransactOpts, _validator)
}

// RenewTime is a paid mutator transaction binding the contract method 0x91c03370.
//
// Solidity: function renewTime(bytes32 fileKey, uint64 _newExpireTime) returns()
func (_FileContract *FileContractTransactor) RenewTime(opts *bind.TransactOpts, fileKey [32]byte, _newExpireTime uint64) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "renewTime", fileKey, _newExpireTime)
}

// RenewTime is a paid mutator transaction binding the contract method 0x91c03370.
//
// Solidity: function renewTime(bytes32 fileKey, uint64 _newExpireTime) returns()
func (_FileContract *FileContractSession) RenewTime(fileKey [32]byte, _newExpireTime uint64) (*types.Transaction, error) {
	return _FileContract.Contract.RenewTime(&_FileContract.TransactOpts, fileKey, _newExpireTime)
}

// RenewTime is a paid mutator transaction binding the contract method 0x91c03370.
//
// Solidity: function renewTime(bytes32 fileKey, uint64 _newExpireTime) returns()
func (_FileContract *FileContractTransactorSession) RenewTime(fileKey [32]byte, _newExpireTime uint64) (*types.Transaction, error) {
	return _FileContract.Contract.RenewTime(&_FileContract.TransactOpts, fileKey, _newExpireTime)
}

// SetPricePerChunk is a paid mutator transaction binding the contract method 0xb226b0e5.
//
// Solidity: function setPricePerChunk(uint256 _newPrice) returns()
func (_FileContract *FileContractTransactor) SetPricePerChunk(opts *bind.TransactOpts, _newPrice *big.Int) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "setPricePerChunk", _newPrice)
}

// SetPricePerChunk is a paid mutator transaction binding the contract method 0xb226b0e5.
//
// Solidity: function setPricePerChunk(uint256 _newPrice) returns()
func (_FileContract *FileContractSession) SetPricePerChunk(_newPrice *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.SetPricePerChunk(&_FileContract.TransactOpts, _newPrice)
}

// SetPricePerChunk is a paid mutator transaction binding the contract method 0xb226b0e5.
//
// Solidity: function setPricePerChunk(uint256 _newPrice) returns()
func (_FileContract *FileContractTransactorSession) SetPricePerChunk(_newPrice *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.SetPricePerChunk(&_FileContract.TransactOpts, _newPrice)
}

// UploadChunk is a paid mutator transaction binding the contract method 0xe6d51227.
//
// Solidity: function uploadChunk(bytes32 fileKey, bytes chunkData, uint256 chunkIndex, bytes32[] merkleProof) returns()
func (_FileContract *FileContractTransactor) UploadChunk(opts *bind.TransactOpts, fileKey [32]byte, chunkData []byte, chunkIndex *big.Int, merkleProof [][32]byte) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "uploadChunk", fileKey, chunkData, chunkIndex, merkleProof)
}

// UploadChunk is a paid mutator transaction binding the contract method 0xe6d51227.
//
// Solidity: function uploadChunk(bytes32 fileKey, bytes chunkData, uint256 chunkIndex, bytes32[] merkleProof) returns()
func (_FileContract *FileContractSession) UploadChunk(fileKey [32]byte, chunkData []byte, chunkIndex *big.Int, merkleProof [][32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.UploadChunk(&_FileContract.TransactOpts, fileKey, chunkData, chunkIndex, merkleProof)
}

// UploadChunk is a paid mutator transaction binding the contract method 0xe6d51227.
//
// Solidity: function uploadChunk(bytes32 fileKey, bytes chunkData, uint256 chunkIndex, bytes32[] merkleProof) returns()
func (_FileContract *FileContractTransactorSession) UploadChunk(fileKey [32]byte, chunkData []byte, chunkIndex *big.Int, merkleProof [][32]byte) (*types.Transaction, error) {
	return _FileContract.Contract.UploadChunk(&_FileContract.TransactOpts, fileKey, chunkData, chunkIndex, merkleProof)
}

// WithdrawAmount is a paid mutator transaction binding the contract method 0x0562b9f7.
//
// Solidity: function withdrawAmount(uint256 amount) returns()
func (_FileContract *FileContractTransactor) WithdrawAmount(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "withdrawAmount", amount)
}

// WithdrawAmount is a paid mutator transaction binding the contract method 0x0562b9f7.
//
// Solidity: function withdrawAmount(uint256 amount) returns()
func (_FileContract *FileContractSession) WithdrawAmount(amount *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.WithdrawAmount(&_FileContract.TransactOpts, amount)
}

// WithdrawAmount is a paid mutator transaction binding the contract method 0x0562b9f7.
//
// Solidity: function withdrawAmount(uint256 amount) returns()
func (_FileContract *FileContractTransactorSession) WithdrawAmount(amount *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.WithdrawAmount(&_FileContract.TransactOpts, amount)
}

// WithdrawFunds is a paid mutator transaction binding the contract method 0x24600fc3.
//
// Solidity: function withdrawFunds() returns()
func (_FileContract *FileContractTransactor) WithdrawFunds(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "withdrawFunds")
}

// WithdrawFunds is a paid mutator transaction binding the contract method 0x24600fc3.
//
// Solidity: function withdrawFunds() returns()
func (_FileContract *FileContractSession) WithdrawFunds() (*types.Transaction, error) {
	return _FileContract.Contract.WithdrawFunds(&_FileContract.TransactOpts)
}

// WithdrawFunds is a paid mutator transaction binding the contract method 0x24600fc3.
//
// Solidity: function withdrawFunds() returns()
func (_FileContract *FileContractTransactorSession) WithdrawFunds() (*types.Transaction, error) {
	return _FileContract.Contract.WithdrawFunds(&_FileContract.TransactOpts)
}

// FileContractChunkUploadedIterator is returned from FilterChunkUploaded and is used to iterate over the raw logs and unpacked data for ChunkUploaded events raised by the FileContract contract.
type FileContractChunkUploadedIterator struct {
	Event *FileContractChunkUploaded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractChunkUploadedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractChunkUploaded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractChunkUploaded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractChunkUploadedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractChunkUploadedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractChunkUploaded represents a ChunkUploaded event raised by the FileContract contract.
type FileContractChunkUploaded struct {
	FileKey    [32]byte
	ChunkIndex *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterChunkUploaded is a free log retrieval operation binding the contract event 0x9194bcee830b22b5e81ccd3c7efb4811ca70dfdd9bbfaa1719d15dfc18c6417d.
//
// Solidity: event ChunkUploaded(bytes32 fileKey, uint256 chunkIndex)
func (_FileContract *FileContractFilterer) FilterChunkUploaded(opts *bind.FilterOpts) (*FileContractChunkUploadedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "ChunkUploaded")
	if err != nil {
		return nil, err
	}
	return &FileContractChunkUploadedIterator{contract: _FileContract.contract, event: "ChunkUploaded", logs: logs, sub: sub}, nil
}

// WatchChunkUploaded is a free log subscription operation binding the contract event 0x9194bcee830b22b5e81ccd3c7efb4811ca70dfdd9bbfaa1719d15dfc18c6417d.
//
// Solidity: event ChunkUploaded(bytes32 fileKey, uint256 chunkIndex)
func (_FileContract *FileContractFilterer) WatchChunkUploaded(opts *bind.WatchOpts, sink chan<- *FileContractChunkUploaded) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "ChunkUploaded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractChunkUploaded)
				if err := _FileContract.contract.UnpackLog(event, "ChunkUploaded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseChunkUploaded is a log parse operation binding the contract event 0x9194bcee830b22b5e81ccd3c7efb4811ca70dfdd9bbfaa1719d15dfc18c6417d.
//
// Solidity: event ChunkUploaded(bytes32 fileKey, uint256 chunkIndex)
func (_FileContract *FileContractFilterer) ParseChunkUploaded(log types.Log) (*FileContractChunkUploaded, error) {
	event := new(FileContractChunkUploaded)
	if err := _FileContract.contract.UnpackLog(event, "ChunkUploaded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractDownloadKeyConfirmedIterator is returned from FilterDownloadKeyConfirmed and is used to iterate over the raw logs and unpacked data for DownloadKeyConfirmed events raised by the FileContract contract.
type FileContractDownloadKeyConfirmedIterator struct {
	Event *FileContractDownloadKeyConfirmed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractDownloadKeyConfirmedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractDownloadKeyConfirmed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractDownloadKeyConfirmed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractDownloadKeyConfirmedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractDownloadKeyConfirmedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractDownloadKeyConfirmed represents a DownloadKeyConfirmed event raised by the FileContract contract.
type FileContractDownloadKeyConfirmed struct {
	DownloadKey [32]byte
	FileKey     [32]byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterDownloadKeyConfirmed is a free log retrieval operation binding the contract event 0x8959843b6f036212c492b345309e20215376803e559952b8445f6703a3c30a95.
//
// Solidity: event DownloadKeyConfirmed(bytes32 downloadKey, bytes32 fileKey)
func (_FileContract *FileContractFilterer) FilterDownloadKeyConfirmed(opts *bind.FilterOpts) (*FileContractDownloadKeyConfirmedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "DownloadKeyConfirmed")
	if err != nil {
		return nil, err
	}
	return &FileContractDownloadKeyConfirmedIterator{contract: _FileContract.contract, event: "DownloadKeyConfirmed", logs: logs, sub: sub}, nil
}

// WatchDownloadKeyConfirmed is a free log subscription operation binding the contract event 0x8959843b6f036212c492b345309e20215376803e559952b8445f6703a3c30a95.
//
// Solidity: event DownloadKeyConfirmed(bytes32 downloadKey, bytes32 fileKey)
func (_FileContract *FileContractFilterer) WatchDownloadKeyConfirmed(opts *bind.WatchOpts, sink chan<- *FileContractDownloadKeyConfirmed) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "DownloadKeyConfirmed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractDownloadKeyConfirmed)
				if err := _FileContract.contract.UnpackLog(event, "DownloadKeyConfirmed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDownloadKeyConfirmed is a log parse operation binding the contract event 0x8959843b6f036212c492b345309e20215376803e559952b8445f6703a3c30a95.
//
// Solidity: event DownloadKeyConfirmed(bytes32 downloadKey, bytes32 fileKey)
func (_FileContract *FileContractFilterer) ParseDownloadKeyConfirmed(log types.Log) (*FileContractDownloadKeyConfirmed, error) {
	event := new(FileContractDownloadKeyConfirmed)
	if err := _FileContract.contract.UnpackLog(event, "DownloadKeyConfirmed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractDownloadKeyGeneratedIterator is returned from FilterDownloadKeyGenerated and is used to iterate over the raw logs and unpacked data for DownloadKeyGenerated events raised by the FileContract contract.
type FileContractDownloadKeyGeneratedIterator struct {
	Event *FileContractDownloadKeyGenerated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractDownloadKeyGeneratedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractDownloadKeyGenerated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractDownloadKeyGenerated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractDownloadKeyGeneratedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractDownloadKeyGeneratedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractDownloadKeyGenerated represents a DownloadKeyGenerated event raised by the FileContract contract.
type FileContractDownloadKeyGenerated struct {
	DownloadKey [32]byte
	FileKey     [32]byte
	User        common.Address
	Amount      *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterDownloadKeyGenerated is a free log retrieval operation binding the contract event 0xa4f37aa3f9a4c270fe8cb516eb0a052f3f342b1e55d39f7acdd37e0498408a23.
//
// Solidity: event DownloadKeyGenerated(bytes32 downloadKey, bytes32 fileKey, address user, uint256 amount)
func (_FileContract *FileContractFilterer) FilterDownloadKeyGenerated(opts *bind.FilterOpts) (*FileContractDownloadKeyGeneratedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "DownloadKeyGenerated")
	if err != nil {
		return nil, err
	}
	return &FileContractDownloadKeyGeneratedIterator{contract: _FileContract.contract, event: "DownloadKeyGenerated", logs: logs, sub: sub}, nil
}

// WatchDownloadKeyGenerated is a free log subscription operation binding the contract event 0xa4f37aa3f9a4c270fe8cb516eb0a052f3f342b1e55d39f7acdd37e0498408a23.
//
// Solidity: event DownloadKeyGenerated(bytes32 downloadKey, bytes32 fileKey, address user, uint256 amount)
func (_FileContract *FileContractFilterer) WatchDownloadKeyGenerated(opts *bind.WatchOpts, sink chan<- *FileContractDownloadKeyGenerated) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "DownloadKeyGenerated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractDownloadKeyGenerated)
				if err := _FileContract.contract.UnpackLog(event, "DownloadKeyGenerated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDownloadKeyGenerated is a log parse operation binding the contract event 0xa4f37aa3f9a4c270fe8cb516eb0a052f3f342b1e55d39f7acdd37e0498408a23.
//
// Solidity: event DownloadKeyGenerated(bytes32 downloadKey, bytes32 fileKey, address user, uint256 amount)
func (_FileContract *FileContractFilterer) ParseDownloadKeyGenerated(log types.Log) (*FileContractDownloadKeyGenerated, error) {
	event := new(FileContractDownloadKeyGenerated)
	if err := _FileContract.contract.UnpackLog(event, "DownloadKeyGenerated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractFileActivatedIterator is returned from FilterFileActivated and is used to iterate over the raw logs and unpacked data for FileActivated events raised by the FileContract contract.
type FileContractFileActivatedIterator struct {
	Event *FileContractFileActivated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractFileActivatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractFileActivated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractFileActivated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractFileActivatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractFileActivatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractFileActivated represents a FileActivated event raised by the FileContract contract.
type FileContractFileActivated struct {
	User    common.Address
	FileKey [32]byte
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterFileActivated is a free log retrieval operation binding the contract event 0x26b943f7553135bdbca07435020f8e290acd169caa2bfe72d3dc512cd7c9ba2f.
//
// Solidity: event FileActivated(address user, bytes32 fileKey)
func (_FileContract *FileContractFilterer) FilterFileActivated(opts *bind.FilterOpts) (*FileContractFileActivatedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "FileActivated")
	if err != nil {
		return nil, err
	}
	return &FileContractFileActivatedIterator{contract: _FileContract.contract, event: "FileActivated", logs: logs, sub: sub}, nil
}

// WatchFileActivated is a free log subscription operation binding the contract event 0x26b943f7553135bdbca07435020f8e290acd169caa2bfe72d3dc512cd7c9ba2f.
//
// Solidity: event FileActivated(address user, bytes32 fileKey)
func (_FileContract *FileContractFilterer) WatchFileActivated(opts *bind.WatchOpts, sink chan<- *FileContractFileActivated) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "FileActivated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractFileActivated)
				if err := _FileContract.contract.UnpackLog(event, "FileActivated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFileActivated is a log parse operation binding the contract event 0x26b943f7553135bdbca07435020f8e290acd169caa2bfe72d3dc512cd7c9ba2f.
//
// Solidity: event FileActivated(address user, bytes32 fileKey)
func (_FileContract *FileContractFilterer) ParseFileActivated(log types.Log) (*FileContractFileActivated, error) {
	event := new(FileContractFileActivated)
	if err := _FileContract.contract.UnpackLog(event, "FileActivated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractFileAddedIterator is returned from FilterFileAdded and is used to iterate over the raw logs and unpacked data for FileAdded events raised by the FileContract contract.
type FileContractFileAddedIterator struct {
	Event *FileContractFileAdded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractFileAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractFileAdded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractFileAdded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractFileAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractFileAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractFileAdded represents a FileAdded event raised by the FileContract contract.
type FileContractFileAdded struct {
	FileKey    [32]byte
	Name       string
	ContentLen uint64
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterFileAdded is a free log retrieval operation binding the contract event 0x7f2db28bb9cdbd41f93b3953994de4298cc665dda981f021c84fc9b4b367f11e.
//
// Solidity: event FileAdded(bytes32 fileKey, string name, uint64 contentLen)
func (_FileContract *FileContractFilterer) FilterFileAdded(opts *bind.FilterOpts) (*FileContractFileAddedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "FileAdded")
	if err != nil {
		return nil, err
	}
	return &FileContractFileAddedIterator{contract: _FileContract.contract, event: "FileAdded", logs: logs, sub: sub}, nil
}

// WatchFileAdded is a free log subscription operation binding the contract event 0x7f2db28bb9cdbd41f93b3953994de4298cc665dda981f021c84fc9b4b367f11e.
//
// Solidity: event FileAdded(bytes32 fileKey, string name, uint64 contentLen)
func (_FileContract *FileContractFilterer) WatchFileAdded(opts *bind.WatchOpts, sink chan<- *FileContractFileAdded) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "FileAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractFileAdded)
				if err := _FileContract.contract.UnpackLog(event, "FileAdded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFileAdded is a log parse operation binding the contract event 0x7f2db28bb9cdbd41f93b3953994de4298cc665dda981f021c84fc9b4b367f11e.
//
// Solidity: event FileAdded(bytes32 fileKey, string name, uint64 contentLen)
func (_FileContract *FileContractFilterer) ParseFileAdded(log types.Log) (*FileContractFileAdded, error) {
	event := new(FileContractFileAdded)
	if err := _FileContract.contract.UnpackLog(event, "FileAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractFileDeletedIterator is returned from FilterFileDeleted and is used to iterate over the raw logs and unpacked data for FileDeleted events raised by the FileContract contract.
type FileContractFileDeletedIterator struct {
	Event *FileContractFileDeleted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractFileDeletedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractFileDeleted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractFileDeleted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractFileDeletedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractFileDeletedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractFileDeleted represents a FileDeleted event raised by the FileContract contract.
type FileContractFileDeleted struct {
	FileKey [32]byte
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterFileDeleted is a free log retrieval operation binding the contract event 0x88efefb1eaec3d3a0e08ed74bc99a3407d454b5da66112fed68b20d54837ccae.
//
// Solidity: event FileDeleted(bytes32 fileKey)
func (_FileContract *FileContractFilterer) FilterFileDeleted(opts *bind.FilterOpts) (*FileContractFileDeletedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "FileDeleted")
	if err != nil {
		return nil, err
	}
	return &FileContractFileDeletedIterator{contract: _FileContract.contract, event: "FileDeleted", logs: logs, sub: sub}, nil
}

// WatchFileDeleted is a free log subscription operation binding the contract event 0x88efefb1eaec3d3a0e08ed74bc99a3407d454b5da66112fed68b20d54837ccae.
//
// Solidity: event FileDeleted(bytes32 fileKey)
func (_FileContract *FileContractFilterer) WatchFileDeleted(opts *bind.WatchOpts, sink chan<- *FileContractFileDeleted) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "FileDeleted")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractFileDeleted)
				if err := _FileContract.contract.UnpackLog(event, "FileDeleted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFileDeleted is a log parse operation binding the contract event 0x88efefb1eaec3d3a0e08ed74bc99a3407d454b5da66112fed68b20d54837ccae.
//
// Solidity: event FileDeleted(bytes32 fileKey)
func (_FileContract *FileContractFilterer) ParseFileDeleted(log types.Log) (*FileContractFileDeleted, error) {
	event := new(FileContractFileDeleted)
	if err := _FileContract.contract.UnpackLog(event, "FileDeleted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractFileLockedIterator is returned from FilterFileLocked and is used to iterate over the raw logs and unpacked data for FileLocked events raised by the FileContract contract.
type FileContractFileLockedIterator struct {
	Event *FileContractFileLocked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractFileLockedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractFileLocked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractFileLocked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractFileLockedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractFileLockedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractFileLocked represents a FileLocked event raised by the FileContract contract.
type FileContractFileLocked struct {
	FileKey [32]byte
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterFileLocked is a free log retrieval operation binding the contract event 0x45987cd84c3c0ec7c3b91dde29affe01363c3777aec1247553883b2431747330.
//
// Solidity: event FileLocked(bytes32 fileKey)
func (_FileContract *FileContractFilterer) FilterFileLocked(opts *bind.FilterOpts) (*FileContractFileLockedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "FileLocked")
	if err != nil {
		return nil, err
	}
	return &FileContractFileLockedIterator{contract: _FileContract.contract, event: "FileLocked", logs: logs, sub: sub}, nil
}

// WatchFileLocked is a free log subscription operation binding the contract event 0x45987cd84c3c0ec7c3b91dde29affe01363c3777aec1247553883b2431747330.
//
// Solidity: event FileLocked(bytes32 fileKey)
func (_FileContract *FileContractFilterer) WatchFileLocked(opts *bind.WatchOpts, sink chan<- *FileContractFileLocked) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "FileLocked")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractFileLocked)
				if err := _FileContract.contract.UnpackLog(event, "FileLocked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFileLocked is a log parse operation binding the contract event 0x45987cd84c3c0ec7c3b91dde29affe01363c3777aec1247553883b2431747330.
//
// Solidity: event FileLocked(bytes32 fileKey)
func (_FileContract *FileContractFilterer) ParseFileLocked(log types.Log) (*FileContractFileLocked, error) {
	event := new(FileContractFileLocked)
	if err := _FileContract.contract.UnpackLog(event, "FileLocked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractFundsWithdrawnIterator is returned from FilterFundsWithdrawn and is used to iterate over the raw logs and unpacked data for FundsWithdrawn events raised by the FileContract contract.
type FileContractFundsWithdrawnIterator struct {
	Event *FileContractFundsWithdrawn // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractFundsWithdrawnIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractFundsWithdrawn)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractFundsWithdrawn)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractFundsWithdrawnIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractFundsWithdrawnIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractFundsWithdrawn represents a FundsWithdrawn event raised by the FileContract contract.
type FileContractFundsWithdrawn struct {
	Owner  common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterFundsWithdrawn is a free log retrieval operation binding the contract event 0xeaff4b37086828766ad3268786972c0cd24259d4c87a80f9d3963a3c3d999b0d.
//
// Solidity: event FundsWithdrawn(address owner, uint256 amount)
func (_FileContract *FileContractFilterer) FilterFundsWithdrawn(opts *bind.FilterOpts) (*FileContractFundsWithdrawnIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "FundsWithdrawn")
	if err != nil {
		return nil, err
	}
	return &FileContractFundsWithdrawnIterator{contract: _FileContract.contract, event: "FundsWithdrawn", logs: logs, sub: sub}, nil
}

// WatchFundsWithdrawn is a free log subscription operation binding the contract event 0xeaff4b37086828766ad3268786972c0cd24259d4c87a80f9d3963a3c3d999b0d.
//
// Solidity: event FundsWithdrawn(address owner, uint256 amount)
func (_FileContract *FileContractFilterer) WatchFundsWithdrawn(opts *bind.WatchOpts, sink chan<- *FileContractFundsWithdrawn) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "FundsWithdrawn")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractFundsWithdrawn)
				if err := _FileContract.contract.UnpackLog(event, "FundsWithdrawn", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseFundsWithdrawn is a log parse operation binding the contract event 0xeaff4b37086828766ad3268786972c0cd24259d4c87a80f9d3963a3c3d999b0d.
//
// Solidity: event FundsWithdrawn(address owner, uint256 amount)
func (_FileContract *FileContractFilterer) ParseFundsWithdrawn(log types.Log) (*FileContractFundsWithdrawn, error) {
	event := new(FileContractFundsWithdrawn)
	if err := _FileContract.contract.UnpackLog(event, "FundsWithdrawn", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractPaymentReceivedIterator is returned from FilterPaymentReceived and is used to iterate over the raw logs and unpacked data for PaymentReceived events raised by the FileContract contract.
type FileContractPaymentReceivedIterator struct {
	Event *FileContractPaymentReceived // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractPaymentReceivedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractPaymentReceived)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractPaymentReceived)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractPaymentReceivedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractPaymentReceivedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractPaymentReceived represents a PaymentReceived event raised by the FileContract contract.
type FileContractPaymentReceived struct {
	FileKey       [32]byte
	Payer         common.Address
	Amount        *big.Int
	DownloadCount *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterPaymentReceived is a free log retrieval operation binding the contract event 0xb84982556d7cb15bbbde57cf4d92e7e35098afd173f6b52783916ffd21f49fab.
//
// Solidity: event PaymentReceived(bytes32 fileKey, address payer, uint256 amount, uint256 downloadCount)
func (_FileContract *FileContractFilterer) FilterPaymentReceived(opts *bind.FilterOpts) (*FileContractPaymentReceivedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "PaymentReceived")
	if err != nil {
		return nil, err
	}
	return &FileContractPaymentReceivedIterator{contract: _FileContract.contract, event: "PaymentReceived", logs: logs, sub: sub}, nil
}

// WatchPaymentReceived is a free log subscription operation binding the contract event 0xb84982556d7cb15bbbde57cf4d92e7e35098afd173f6b52783916ffd21f49fab.
//
// Solidity: event PaymentReceived(bytes32 fileKey, address payer, uint256 amount, uint256 downloadCount)
func (_FileContract *FileContractFilterer) WatchPaymentReceived(opts *bind.WatchOpts, sink chan<- *FileContractPaymentReceived) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "PaymentReceived")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractPaymentReceived)
				if err := _FileContract.contract.UnpackLog(event, "PaymentReceived", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePaymentReceived is a log parse operation binding the contract event 0xb84982556d7cb15bbbde57cf4d92e7e35098afd173f6b52783916ffd21f49fab.
//
// Solidity: event PaymentReceived(bytes32 fileKey, address payer, uint256 amount, uint256 downloadCount)
func (_FileContract *FileContractFilterer) ParsePaymentReceived(log types.Log) (*FileContractPaymentReceived, error) {
	event := new(FileContractPaymentReceived)
	if err := _FileContract.contract.UnpackLog(event, "PaymentReceived", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractStorageConfirmedIterator is returned from FilterStorageConfirmed and is used to iterate over the raw logs and unpacked data for StorageConfirmed events raised by the FileContract contract.
type FileContractStorageConfirmedIterator struct {
	Event *FileContractStorageConfirmed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FileContractStorageConfirmedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractStorageConfirmed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FileContractStorageConfirmed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FileContractStorageConfirmedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractStorageConfirmedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractStorageConfirmed represents a StorageConfirmed event raised by the FileContract contract.
type FileContractStorageConfirmed struct {
	DownloadKey          [32]byte
	StorageServer        common.Address
	CurrentConfirmations *big.Int
	Raw                  types.Log // Blockchain specific contextual infos
}

// FilterStorageConfirmed is a free log retrieval operation binding the contract event 0x811c7f8ac0d6213e0872f4295202cd7f40f596dbdc894e6e414a50909df7b7d8.
//
// Solidity: event StorageConfirmed(bytes32 downloadKey, address storageServer, uint256 currentConfirmations)
func (_FileContract *FileContractFilterer) FilterStorageConfirmed(opts *bind.FilterOpts) (*FileContractStorageConfirmedIterator, error) {

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "StorageConfirmed")
	if err != nil {
		return nil, err
	}
	return &FileContractStorageConfirmedIterator{contract: _FileContract.contract, event: "StorageConfirmed", logs: logs, sub: sub}, nil
}

// WatchStorageConfirmed is a free log subscription operation binding the contract event 0x811c7f8ac0d6213e0872f4295202cd7f40f596dbdc894e6e414a50909df7b7d8.
//
// Solidity: event StorageConfirmed(bytes32 downloadKey, address storageServer, uint256 currentConfirmations)
func (_FileContract *FileContractFilterer) WatchStorageConfirmed(opts *bind.WatchOpts, sink chan<- *FileContractStorageConfirmed) (event.Subscription, error) {

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "StorageConfirmed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractStorageConfirmed)
				if err := _FileContract.contract.UnpackLog(event, "StorageConfirmed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseStorageConfirmed is a log parse operation binding the contract event 0x811c7f8ac0d6213e0872f4295202cd7f40f596dbdc894e6e414a50909df7b7d8.
//
// Solidity: event StorageConfirmed(bytes32 downloadKey, address storageServer, uint256 currentConfirmations)
func (_FileContract *FileContractFilterer) ParseStorageConfirmed(log types.Log) (*FileContractStorageConfirmed, error) {
	event := new(FileContractStorageConfirmed)
	if err := _FileContract.contract.UnpackLog(event, "StorageConfirmed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
