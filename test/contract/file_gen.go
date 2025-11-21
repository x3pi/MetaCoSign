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

// FileContractMetaData contains all meta data concerning the FileContract contract.
var FileContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_id\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"_message\",\"type\":\"string\"}],\"name\":\"ping\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"string\",\"name\":\"message\",\"type\":\"string\"}],\"name\":\"Ping\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_v\",\"type\":\"uint256\"}],\"name\":\"setValue\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"setter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"ValueSet\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"lastValue\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
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

// LastValue is a free data retrieval call binding the contract method 0x43183834.
//
// Solidity: function lastValue() view returns(uint256)
func (_FileContract *FileContractCaller) LastValue(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _FileContract.contract.Call(opts, &out, "lastValue")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastValue is a free data retrieval call binding the contract method 0x43183834.
//
// Solidity: function lastValue() view returns(uint256)
func (_FileContract *FileContractSession) LastValue() (*big.Int, error) {
	return _FileContract.Contract.LastValue(&_FileContract.CallOpts)
}

// LastValue is a free data retrieval call binding the contract method 0x43183834.
//
// Solidity: function lastValue() view returns(uint256)
func (_FileContract *FileContractCallerSession) LastValue() (*big.Int, error) {
	return _FileContract.Contract.LastValue(&_FileContract.CallOpts)
}

// Ping is a paid mutator transaction binding the contract method 0xcba2a316.
//
// Solidity: function ping(uint256 _id, string _message) returns()
func (_FileContract *FileContractTransactor) Ping(opts *bind.TransactOpts, _id *big.Int, _message string) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "ping", _id, _message)
}

// Ping is a paid mutator transaction binding the contract method 0xcba2a316.
//
// Solidity: function ping(uint256 _id, string _message) returns()
func (_FileContract *FileContractSession) Ping(_id *big.Int, _message string) (*types.Transaction, error) {
	return _FileContract.Contract.Ping(&_FileContract.TransactOpts, _id, _message)
}

// Ping is a paid mutator transaction binding the contract method 0xcba2a316.
//
// Solidity: function ping(uint256 _id, string _message) returns()
func (_FileContract *FileContractTransactorSession) Ping(_id *big.Int, _message string) (*types.Transaction, error) {
	return _FileContract.Contract.Ping(&_FileContract.TransactOpts, _id, _message)
}

// SetValue is a paid mutator transaction binding the contract method 0x55241077.
//
// Solidity: function setValue(uint256 _v) returns()
func (_FileContract *FileContractTransactor) SetValue(opts *bind.TransactOpts, _v *big.Int) (*types.Transaction, error) {
	return _FileContract.contract.Transact(opts, "setValue", _v)
}

// SetValue is a paid mutator transaction binding the contract method 0x55241077.
//
// Solidity: function setValue(uint256 _v) returns()
func (_FileContract *FileContractSession) SetValue(_v *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.SetValue(&_FileContract.TransactOpts, _v)
}

// SetValue is a paid mutator transaction binding the contract method 0x55241077.
//
// Solidity: function setValue(uint256 _v) returns()
func (_FileContract *FileContractTransactorSession) SetValue(_v *big.Int) (*types.Transaction, error) {
	return _FileContract.Contract.SetValue(&_FileContract.TransactOpts, _v)
}

// FileContractPingIterator is returned from FilterPing and is used to iterate over the raw logs and unpacked data for Ping events raised by the FileContract contract.
type FileContractPingIterator struct {
	Event *FileContractPing // Event containing the contract specifics and raw log

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
func (it *FileContractPingIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractPing)
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
		it.Event = new(FileContractPing)
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
func (it *FileContractPingIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractPingIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractPing represents a Ping event raised by the FileContract contract.
type FileContractPing struct {
	Id      *big.Int
	Message string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPing is a free log retrieval operation binding the contract event 0xa08082c7663f884e3c4d325ad1de149f6e167a84556be205103c16b1595d22cc.
//
// Solidity: event Ping(uint256 indexed id, string message)
func (_FileContract *FileContractFilterer) FilterPing(opts *bind.FilterOpts, id []*big.Int) (*FileContractPingIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "Ping", idRule)
	if err != nil {
		return nil, err
	}
	return &FileContractPingIterator{contract: _FileContract.contract, event: "Ping", logs: logs, sub: sub}, nil
}

// WatchPing is a free log subscription operation binding the contract event 0xa08082c7663f884e3c4d325ad1de149f6e167a84556be205103c16b1595d22cc.
//
// Solidity: event Ping(uint256 indexed id, string message)
func (_FileContract *FileContractFilterer) WatchPing(opts *bind.WatchOpts, sink chan<- *FileContractPing, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "Ping", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractPing)
				if err := _FileContract.contract.UnpackLog(event, "Ping", log); err != nil {
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

// ParsePing is a log parse operation binding the contract event 0xa08082c7663f884e3c4d325ad1de149f6e167a84556be205103c16b1595d22cc.
//
// Solidity: event Ping(uint256 indexed id, string message)
func (_FileContract *FileContractFilterer) ParsePing(log types.Log) (*FileContractPing, error) {
	event := new(FileContractPing)
	if err := _FileContract.contract.UnpackLog(event, "Ping", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// FileContractValueSetIterator is returned from FilterValueSet and is used to iterate over the raw logs and unpacked data for ValueSet events raised by the FileContract contract.
type FileContractValueSetIterator struct {
	Event *FileContractValueSet // Event containing the contract specifics and raw log

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
func (it *FileContractValueSetIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FileContractValueSet)
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
		it.Event = new(FileContractValueSet)
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
func (it *FileContractValueSetIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FileContractValueSetIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FileContractValueSet represents a ValueSet event raised by the FileContract contract.
type FileContractValueSet struct {
	Setter common.Address
	Value  *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterValueSet is a free log retrieval operation binding the contract event 0xf3f57717dff9f5f10af315efdbfadc60c42152c11fc0c3c413bbfbdc661f143c.
//
// Solidity: event ValueSet(address indexed setter, uint256 value)
func (_FileContract *FileContractFilterer) FilterValueSet(opts *bind.FilterOpts, setter []common.Address) (*FileContractValueSetIterator, error) {

	var setterRule []interface{}
	for _, setterItem := range setter {
		setterRule = append(setterRule, setterItem)
	}

	logs, sub, err := _FileContract.contract.FilterLogs(opts, "ValueSet", setterRule)
	if err != nil {
		return nil, err
	}
	return &FileContractValueSetIterator{contract: _FileContract.contract, event: "ValueSet", logs: logs, sub: sub}, nil
}

// WatchValueSet is a free log subscription operation binding the contract event 0xf3f57717dff9f5f10af315efdbfadc60c42152c11fc0c3c413bbfbdc661f143c.
//
// Solidity: event ValueSet(address indexed setter, uint256 value)
func (_FileContract *FileContractFilterer) WatchValueSet(opts *bind.WatchOpts, sink chan<- *FileContractValueSet, setter []common.Address) (event.Subscription, error) {

	var setterRule []interface{}
	for _, setterItem := range setter {
		setterRule = append(setterRule, setterItem)
	}

	logs, sub, err := _FileContract.contract.WatchLogs(opts, "ValueSet", setterRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FileContractValueSet)
				if err := _FileContract.contract.UnpackLog(event, "ValueSet", log); err != nil {
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

// ParseValueSet is a log parse operation binding the contract event 0xf3f57717dff9f5f10af315efdbfadc60c42152c11fc0c3c413bbfbdc661f143c.
//
// Solidity: event ValueSet(address indexed setter, uint256 value)
func (_FileContract *FileContractFilterer) ParseValueSet(log types.Log) (*FileContractValueSet, error) {
	event := new(FileContractValueSet)
	if err := _FileContract.contract.UnpackLog(event, "ValueSet", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
