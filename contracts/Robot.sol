// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;


import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";


contract UniversalRobotBus is Initializable, UUPSUpgradeable {
    // --- STORAGE ---
    mapping(address => bool) public owners;
    address[] public ownerList; // Thêm để dễ quản lý danh sách node 3060

    // Event duy nhất mang tính tổng quát cao
    event EmitSentence(
        bytes32 sessionId,
        bytes32 actionId,
        address operator,
        bytes data
    );

    modifier onlyOwner() {
        require(owners[msg.sender], "Not owner");
        _;
    }

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize() public virtual initializer {
        __UUPSUpgradeable_init();
        owners[msg.sender] = true;
        ownerList.push(msg.sender);
    }
    // Hàm bắt buộc của UUPS, thêm virtual để sau này thay đổi cơ chế phân quyền nâng cấp
    function _authorizeUpgrade(address newImplementation) internal virtual override onlyOwner {}

    // --- CORE DISPATCHER ---
    /**
     * @dev Đã thêm virtual: Tương lai bạn có thể override để thêm:
     * 1. Cơ chế thu phí (fee).
     * 2. Kiểm tra điều kiện Robot (status check).
     * 3. Ghi log bổ sung vào Storage thay vì chỉ emit Event.
     */
    function dispatch(
        bytes32 sessionId,
        bytes32 actionId,
        bytes calldata data
    ) external virtual onlyOwner {
        _beforeDispatch(sessionId, actionId, data); // Hook để mở rộng
        emit EmitSentence(sessionId, actionId, msg.sender, data);
        _afterDispatch(sessionId, actionId, data);  // Hook để mở rộng
    }

    // --- HOOKS (Dùng để override ở các bản nâng cấp sau) ---
    function _beforeDispatch(bytes32 sessionId, bytes32 actionId, bytes calldata data) internal virtual {}
    function _afterDispatch(bytes32 sessionId, bytes32 actionId, bytes calldata data) internal virtual {}

    // --- OWNER MANAGEMENT ---
    function setOwner(address _owner, bool _status) external virtual onlyOwner {
        owners[_owner] = _status;
        if(_status) {
            ownerList.push(_owner);
        }
    }
    function getOwnerList() external view virtual returns (address[] memory) {
        return ownerList;
    }
}