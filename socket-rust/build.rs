fn main() {
    // Thêm google protobuf includes
    let mut config = prost_build::Config::new();
     // Thư mục xuất ra file .rs (nên giữ nguyên trong src/proto)
    config.out_dir("src/proto");
    
    // Chỉ định đường dẫn chính xác tới file .proto và thư mục include
    config
        .compile_protos(&["src/proto/validator.proto"], &["src/proto/"])
        .unwrap();
}
