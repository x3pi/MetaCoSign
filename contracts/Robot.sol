// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;

contract RobotContract {
    struct Session {
        address robotAddress;
        uint256 sessionId;
        uint256 createdAt;
        bool isActive;
        string[] sentences; 
    }
    
    mapping(uint256 => Session) public sessions;
    mapping(address => uint256[]) public robotSessions;
    
    event SessionCreated(uint256 sessionId, address robot, uint256 timestamp);
    event SentenceEmitted(uint256 sessionId, uint256 sentenceIndex, string sentence, uint256 timestamp);
    event AIRequest(uint256 sessionId, bytes requestData, uint256 timestamp);
    
    function createSession(
        uint256 sessionId,
        address robotAddress,
        bytes calldata requestData
    ) external {
        require(!sessions[sessionId].isActive, "Session already exists");

        // Cách tối ưu: Truy cập trực tiếp vào storage để tránh lỗi khởi tạo mảng
        Session storage newSession = sessions[sessionId];
        newSession.robotAddress = robotAddress;
        newSession.sessionId = sessionId;
        newSession.createdAt = block.timestamp;
        newSession.isActive = true;
        // Không cần khởi tạo sentences, nó mặc định là mảng rỗng

        robotSessions[robotAddress].push(sessionId);

        emit SessionCreated(sessionId, robotAddress, block.timestamp);
        emit AIRequest(sessionId, requestData, block.timestamp);
    }
    
    function emitSentence(
        uint256 sessionId,
        uint256 sentenceIndex,
        string calldata sentence
    ) external {
        require(sessions[sessionId].isActive, "Session not active");
        // Lưu ý: msg.sender phải là robotAddress đã đăng ký
        require(sessions[sessionId].robotAddress == msg.sender, "Unauthorized");
        
        sessions[sessionId].sentences.push(sentence);
        
        emit SentenceEmitted(sessionId, sentenceIndex, sentence, block.timestamp);
    }
    
    // Hàm view để lấy danh sách sentences (vì mapping public không trả về mảng trong struct)
    function getSessionSentences(uint256 sessionId) external view returns (string[] memory) {
        return sessions[sessionId].sentences;
    }
}