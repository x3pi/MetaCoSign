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
    address: "0x68dC15a5734fBd4155158A444F5644d88D98f616",
  },

  // Thêm các contracts khác ở đây nếu cần
  // Ví dụ:
  // TokenContract: {
  //   abi: TokenAbi,
  //   address: "0x...",
  // },
};
