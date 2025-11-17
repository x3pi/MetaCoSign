// File: src/client_handler.rs
use crate::proto::validator::{request, response, BlockRequest, Request, Response, StatusRequest, ValidatorList};
use prost::Message;
use std::io::{Read, Write};
use std::os::unix::net::{ UnixStream};
// Ví dụ về hàm xử lý một yêu cầu BlockRequest
pub fn handle_block_request(
    stream: &mut UnixStream,
    socket_path: &str,
    block_number: u64,
) -> Result<(), Box<dyn std::error::Error>> {
    println!("[{}] Gửi BlockRequest cho block {}", socket_path, block_number);

    let block_req = BlockRequest { block_number };
    let wrapped_request = Request {
        payload: Some(request::Payload::BlockRequest(block_req)),
    };

    // 3. Gửi message đã bọc đi
    write_message(stream, &wrapped_request)?;

    // 4. Nhận về message Response bao bọc
    let wrapped_response: Response = read_message(stream)?;

    // 5. Dùng `match` để xử lý payload bên trong response
    if let Some(payload) = wrapped_response.payload {
        match payload {
            response::Payload::ValidatorList(validator_list) => {
                // Gọi hàm in ra như cũ
                println!(
                    "___________________[{}] Nhận được ValidatorList với {} validators cho block {}________",
                    socket_path,
                    validator_list.validators.len(),
                    block_number
                );
                print_validators(&validator_list, socket_path);
            }
            _ => {
                eprintln!("[{}] Nhận được loại response không mong muốn!", socket_path);
            }
        }
    } else {
        eprintln!("[{}] Nhận được response rỗng!", socket_path);
    }

    Ok(())
}
pub fn handle_status_request(
    stream: &mut UnixStream,
    socket_path: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    // 1. Tạo và bọc request
    let wrapped_request = Request {
        payload: Some(request::Payload::StatusRequest(StatusRequest {})),
    };
    // 2. Gửi đi
    write_message(stream, &wrapped_request)?;
    
    // 3. Nhận về
    let wrapped_response: Response = read_message(stream)?;
    // 4. Xử lý response
    if let Some(payload) = wrapped_response.payload {
        match payload {
            response::Payload::ServerStatus(status) => {
                println!(
                    "++++++++++++++++++++[{}] Nhận được ServerStatus: '{}', uptime: {}s ++++++++++++++++",
                    socket_path, status.status_message, status.uptime_seconds
                );
            }
            _ => {
                 eprintln!("[{}] Nhận được loại response không mong muốn cho StatusRequest!", socket_path);
            }
        }
    }
    
    Ok(())
}
fn print_validators(validator_list: &ValidatorList, socket_path: &str) {
    for (i, validator) in validator_list.validators.iter().enumerate() {
        println!("\n[{}] Validator #{}", socket_path, i + 1);
        println!("  Address:                      {}", validator.address);
        println!("  Name:                         {}", validator.name);
        println!("  Description:                  {}", validator.description);
        println!("  Website:                      {}", validator.website);
        println!("  Is Jailed:                    {}", validator.is_jailed);
        println!(
            "  Commission Rate:              {}",
            validator.commission_rate
        );
        println!(
            "  Min Self Delegation:          {}",
            validator.min_self_delegation
        );
        println!("  Primary Address:              {}", validator.primary_address);
        println!("  Worker Address:               {}", validator.worker_address);
        println!("  P2P Address:                  {}", validator.p2p_address);
        println!("   pubkey_bls:                  {}", validator.pubkey_bls);
        println!("   pubkey_secp:                 {}", validator.pubkey_secp);
        println!(
            "  Total Staked Amount:          {}",
            validator.total_staked_amount
        );
    }

    println!(
        "[{}] Tổng cộng: {} validators",
        socket_path,
        validator_list.validators.len()
    );
}
fn write_message<W: Write, M: Message>(stream: &mut W, message: &M) -> std::io::Result<()> {
    // Serialize message
    let msg_bytes = message.encode_to_vec();
    let msg_len = msg_bytes.len() as u32;

    // Gửi độ dài của message (4 bytes, big-endian)
    stream.write_all(&msg_len.to_be_bytes())?;

    // Gửi data của message
    stream.write_all(&msg_bytes)?;

    Ok(())
}
fn read_message<R: Read, M: Message + Default>(stream: &mut R) -> Result<M, Box<dyn std::error::Error>> {
    // Đọc độ dài của response (4 bytes)
    let mut len_buf = [0u8; 4];
    stream.read_exact(&mut len_buf)?;
    let response_len = u32::from_be_bytes(len_buf) as usize;

    // Đọc data của response
    let mut response_buf = vec![0u8; response_len];
    stream.read_exact(&mut response_buf)?;

    // Deserialize message
    let message = M::decode(&response_buf[..])?;
    Ok(message)
}
