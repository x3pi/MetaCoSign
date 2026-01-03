// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

contract UniversalRobotBus is Initializable, UUPSUpgradeable {
    // --- STORAGE ---
    mapping(address => bool) public owners;
    address[] public ownerList; 

    event EmitSentence(
        bytes32 sessionId,
        bytes32 actionId,
        address operator,
        bytes data
    );
    
    event EmitError(
        bytes32 txHash,
        string message
    );

    modifier onlyOwner() {
        require(owners[msg.sender], "Not owner");
        _;
    }

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize() public initializer {
        __UUPSUpgradeable_init();
        owners[msg.sender] = true;
        ownerList.push(msg.sender);
    }

    function _authorizeUpgrade(address newImplementation) internal override onlyOwner {}

    // --- CORE DISPATCHER ---
    function dispatch(
        bytes32 sessionId,
        bytes32 actionId,
        bytes calldata data,
        uint256 timestamp,
        bytes calldata sig
    ) external virtual {
        _beforeDispatch(sessionId, actionId, data);
        emit EmitSentence(sessionId, actionId, msg.sender, data);
        _afterDispatch(sessionId, actionId, data);
    }

    function _beforeDispatch(bytes32 sessionId, bytes32 actionId, bytes calldata data) internal virtual {}
    function _afterDispatch(bytes32 sessionId, bytes32 actionId, bytes calldata data) internal virtual {}

    // SỬA LỖI: Thêm 's' vào bytes32 và truyền đủ tham số cho emit
    function emitError(bytes32 txHash, string memory message) external virtual {
        emit EmitError(txHash, message);
    }
    function getDataByTxhash(bytes32 txHash) external view virtual {
    }
    // --- OWNER MANAGEMENT ---
    function setOwner(address _owner, bool _status) external virtual onlyOwner {
        if (_status && !owners[_owner]) {
            owners[_owner] = true;
            ownerList.push(_owner);
        } else if (!_status && owners[_owner]) {
            owners[_owner] = false;
        }
    }
    function getOwnerList() external view virtual returns (address[] memory) {
        return ownerList;
    }
}