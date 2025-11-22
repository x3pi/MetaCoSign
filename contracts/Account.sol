// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;

contract AccountManager {
    event AccountConfirmed(address account, uint time,string message);
    event RegisterBls(address account, uint time ,bytes publicKey,string message);
    function setBlsPublicKey(bytes memory _publicKey) external {
        address account;
        uint time;
        string memory message;
        emit RegisterBls(account, time,_publicKey, message );
    }
    function setAccountType(uint8 _type) external {
    }
    function getAllAccount(bytes memory _sign, bytes memory _publicKeyBls, uint _time, uint _page, uint _pageSize, bool _isConfirm) external {
    }
    function getNotifications(address _account,uint page, uint pageSize)external { 

    }
    function confirmAccount(address _account, uint time,bytes memory _sign) external {
        string memory message;
        emit AccountConfirmed(_account, time, message );
    }
}
