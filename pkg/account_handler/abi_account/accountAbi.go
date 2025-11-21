package abi_account

const AccountABI = `[
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "address",
				"name": "account",
				"type": "address"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "time",
				"type": "uint256"
			}
		],
		"name": "AccountConfirmed",
		"type": "event"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "_account",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "time",
				"type": "uint256"
			},
			{
				"internalType": "bytes",
				"name": "_sign",
				"type": "bytes"
			}
		],
		"name": "confirmAccount",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "bytes",
				"name": "_sign",
				"type": "bytes"
			},
			{
				"internalType": "bytes",
				"name": "_publicKeyBls",
				"type": "bytes"
			},
			{
				"internalType": "uint256",
				"name": "_time",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "_page",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "_pageSize",
				"type": "uint256"
			},
			{
				"internalType": "bool",
				"name": "_isConfirm",
				"type": "bool"
			}
		],
		"name": "getAllAccount",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "uint8",
				"name": "_type",
				"type": "uint8"
			}
		],
		"name": "setAccountType",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "bytes",
				"name": "_publicKey",
				"type": "bytes"
			}
		],
		"name": "setBlsPublicKey",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`
