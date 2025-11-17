use std::thread;
mod client_handler;
mod proto;
mod server;
fn main() {
    const SOCKET_PATHS: [&str; 2] = ["/tmp/rust-go.sock_1", "/tmp/rust-go.sock_2"];
    let mut handles = vec![];


    for path in SOCKET_PATHS {
        let handle = thread::spawn(move || {
            // Gọi hàm từ module server
            server::start_connector(path);
        });
        handles.push(handle);
    }

    // Chờ tất cả listener kết thúc
    for handle in handles {
        handle.join().unwrap();
    }
}