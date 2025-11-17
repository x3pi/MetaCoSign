// File: src/server.rs

use std::os::unix::net::UnixStream;
use std::thread;
use std::time::Duration;

use crate::client_handler::{handle_block_request, handle_status_request};

pub fn start_connector(socket_path: &'static str) {
    loop {
        println!("[Rust Client] Đang thử kết nối tới {}...", socket_path);
        // Thử kết nối tới socket
        match UnixStream::connect(socket_path) {
            Ok(mut stream) => {
                let status = handle_status_request(&mut stream, socket_path);
                if let Err(e) = status {
                    eprintln!(
                        "[{}] Lỗi khi xử lý StatusRequest: {}. Mất kết nối tới Go server.",
                        socket_path, e
                    );
                    break; // Thoát khỏi vòng lặp xử lý, quay lại vòng lặp kết nối
                }
                let mut block_number: u64 = 0;
                // Vòng lặp gửi request/response trên kết nối hiện tại
                loop {
                    let result = handle_block_request(&mut stream, socket_path, block_number);
                    if let Err(e) = result {
                        eprintln!(
                            "[{}] Lỗi khi xử lý block {}: {}. Mất kết nối tới Go server.",
                            socket_path, block_number, e
                        );
                        break; // Thoát khỏi vòng lặp xử lý, quay lại vòng lặp kết nối
                    }
                    block_number += 1;
                    thread::sleep(Duration::from_secs(5));
                }
            }
            Err(e) => {
                eprintln!(
                    "[Rust Client] Không thể kết nối tới {}: {}. Thử lại sau 2 giây.",
                    socket_path, e
                );
                // Đợi một chút trước khi thử kết nối lại
                thread::sleep(Duration::from_secs(2));
            }
        }
    }
}
