import RobotAbi from "../abi/robotAbi.json";

/**
 * Danh sách contract
 * Mỗi contract gồm:
 * - abi
 * - address
 */
export const contracts = {
  RobotManager: {
    abi: RobotAbi,
    address: "0x78affC3e85CB90e5bFd567703A4bF81Ae81b64A6",
  },

  // Thêm các contracts khác ở đây nếu cần
  // Ví dụ:
  // TokenContract: {
  //   abi: TokenAbi,
  //   address: "0x...",
  // },
};
