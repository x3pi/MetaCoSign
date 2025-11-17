package node

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time" // Đảm bảo import time

	"github.com/meta-node-blockchain/meta-node/pkg/logger"
	// "github.com/google/uuid" // Bỏ comment nếu muốn dùng UUID thay cho timestamp
)

// --- Cấu trúc và Hàm Helper (Giữ nguyên) ---

type CompressionProfile struct {
	Name      string
	Arguments []string
	MinRAMMB  uint64
	MinCores  int
	IsDefault bool
}

func DetermineBestProfile() (*CompressionProfile, error) {
	cpuCount := runtime.NumCPU()
	mmtArg := "-mmt=off"
	if cpuCount >= 2 {
		mmtArg = "-mmt=on"
	}
	return &CompressionProfile{
		Name:      "Default-DynamicMT",
		Arguments: []string{"-mx=5", mmtArg},
		IsDefault: true,
	}, nil
}

// executeExternalCommand thực thi lệnh bên ngoài.
// Sửa đổi để chấp nhận context và log tốt hơn.
func executeExternalCommand(ctx context.Context, commandString string) (string, error) {
	if commandString == "" {
		return "", fmt.Errorf("chuỗi lệnh trống")
	}

	// Sử dụng "sh -c" và CommandContext để hỗ trợ hủy bỏ
	cmd := exec.CommandContext(ctx, "sh", "-c", commandString)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Info("(Snapshot) Đang thực thi lệnh shell: ", commandString)
	err := cmd.Run() // Chạy lệnh shell

	stdoutStr := strings.TrimSpace(stdout.String())
	stderrStr := strings.TrimSpace(stderr.String())

	logMsg := fmt.Sprintf("(Snapshot) Lệnh shell '%s'", commandString)
	if stdoutStr != "" {
		logMsg += fmt.Sprintf("\n  stdout: %s", stdoutStr)
	}
	if stderrStr != "" {
		logMsg += fmt.Sprintf("\n  stderr: %s", stderrStr)
	}

	// Kiểm tra lỗi context trước lỗi command
	if ctx.Err() != nil {
		logger.Error(logMsg) // Log output ngay cả khi context bị hủy
		logger.Error("(Snapshot) Context bị hủy trong khi thực thi lệnh shell.")
		return stdoutStr, fmt.Errorf("context bị hủy: %w (stderr: %s)", ctx.Err(), stderrStr)
	}

	if err != nil {
		logger.Error(logMsg) // Log output khi có lỗi
		return stdoutStr, fmt.Errorf("lỗi thực thi lệnh shell '%s': %w\nstderr: %s", commandString, err, stderrStr)
	}

	if stdoutStr != "" || stderrStr != "" {
		logger.Info(logMsg)
	} else {
		logger.Debug("(Snapshot) Thực thi lệnh shell thành công (không có output).")
	}

	return stdoutStr, nil
}

// run7zCompress thực thi lệnh 7z với các tham số đã cho. (Giữ nguyên)
func run7zCompress(ctx context.Context, sevenZArgs []string, description string) error {
	sevenZPath, err := exec.LookPath("7z")
	if err != nil {
		logger.Error("Lệnh '7z' không tìm thấy trong PATH hệ thống. Hãy đảm bảo 7zip đã được cài đặt.")
		return fmt.Errorf("lệnh '7z' không tìm thấy trong PATH hệ thống: %w", err)
	}

	cmd := exec.CommandContext(ctx, sevenZPath, sevenZArgs...)
	logger.Debug(fmt.Sprintf("(7z) Đang thực thi [%s]: %s", description, cmd.String()))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	if len(stdoutStr) > 0 {
		logger.Debug("(7z) stdout:", stdoutStr)
	}
	if len(stderrStr) > 0 {
		if err != nil {
			logger.Error("(7z) stderr:", stderrStr)
		} else {
			logger.Debug("(7z) stderr:", stderrStr)
		}
	}

	if err != nil {
		logger.Error(fmt.Sprintf("(7z) Lệnh nén/giải nén thất bại [%s]", description))
		return fmt.Errorf("lỗi thực thi 7z (%s): %w\nstderr: %s", strings.Join(sevenZArgs, " "), err, stderrStr)
	}
	logger.Info(fmt.Sprintf("(7z) Thực thi lệnh thành công [%s]", description))
	return nil
}

// findGeneratedParts tìm các file đã tạo bởi 7z. (Giữ nguyên)
func findGeneratedParts(outputPathPrefix string, isSplit bool) ([]string, error) {
	var pattern string
	if isSplit {
		pattern = outputPathPrefix + ".*"
	} else {
		pattern = outputPathPrefix
	}

	generatedParts, globErr := filepath.Glob(pattern)
	if globErr != nil {
		logger.Error(fmt.Sprintf("Lỗi tìm file đã tạo với pattern '%s': %v", pattern, globErr))
		return nil, fmt.Errorf("lỗi tìm file part sau khi nén: %w", globErr)
	}

	if len(generatedParts) == 0 {
		if !isSplit {
			if _, statErr := os.Stat(outputPathPrefix); statErr == nil {
				return []string{outputPathPrefix}, nil
			}
		}
		logger.Error(fmt.Sprintf("Không tìm thấy file nén/part nào khớp pattern '%s'.", pattern))
		return nil, fmt.Errorf("không tìm thấy file nén/part nào khớp '%s' sau khi nén", pattern)
	}
	return generatedParts, nil
}

// --- Hàm Nén/Giải Nén Gốc (Giữ nguyên để tương thích nếu cần) ---
// Có thể giữ lại các hàm gốc hoặc xóa đi nếu không còn dùng đến

// compressFolderAndSplitInternal (Giữ nguyên)
func compressFolderAndSplitInternal(ctx context.Context, sourceDir, outputDir, baseArchiveName string, splitSizeMB int, snapshotUsed bool) ([]string, error) {
	logSource := sourceDir
	if snapshotUsed {
		logSource = fmt.Sprintf("%s (từ snapshot)", sourceDir)
	}
	cleanBaseName := filepath.Base(baseArchiveName)
	if !strings.HasSuffix(cleanBaseName, ".7z") {
		cleanBaseName += ".7z"
	}
	outputPathPrefix := filepath.Join(outputDir, cleanBaseName)

	logger.Info(fmt.Sprintf("Chuẩn bị nén '%s' vào '%s', bắt đầu với '%s', chia nhỏ: %d MB", logSource, outputDir, outputPathPrefix, splitSizeMB))

	info, err := os.Stat(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("lỗi kiểm tra thư mục nguồn '%s': %w", sourceDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("đường dẫn nguồn '%s' không phải là thư mục", sourceDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("lỗi tạo thư mục output '%s': %w", outputDir, err)
	}

	patternToRemove := outputPathPrefix + ".*"
	existingParts, _ := filepath.Glob(patternToRemove)
	if splitSizeMB <= 0 {
		if _, err := os.Stat(outputPathPrefix); err == nil {
			existingParts = append(existingParts, outputPathPrefix)
		}
	}
	for _, f := range existingParts {
		logger.Debug(fmt.Sprintf("Đang xóa file/part nén cũ: %s", f))
		if err := os.Remove(f); err != nil {
			logger.Warn(fmt.Sprintf("Không thể xóa file cũ %s: %v", f, err))
		}
	}

	profile, err := DetermineBestProfile()
	var compressionArgs []string
	if err != nil {
		logger.Warn("Không thể xác định cấu hình nén tốt nhất, dùng mặc định (-mx=5 -mmt=on):", err)
		compressionArgs = []string{"-mx=5", "-mmt=on"}
	} else {
		compressionArgs = profile.Arguments
		logger.Info("Sử dụng cấu hình nén được xác định tự động:", profile.Name, profile.Arguments)
	}

	args := []string{"a", "-y"}
	args = append(args, compressionArgs...)

	isSplit := false
	if splitSizeMB > 0 {
		isSplit = true
		volumeSize := strconv.Itoa(splitSizeMB) + "m"
		args = append(args, "-v"+volumeSize)
		logger.Info(fmt.Sprintf("Chia archive thành các phần dung lượng: %s", volumeSize))
	}

	args = append(args, outputPathPrefix, sourceDir)

	err = run7zCompress(ctx, args, fmt.Sprintf("CompressFolderAndSplit for %s", sourceDir))
	if err != nil {
		return nil, err
	}

	generatedParts, err := findGeneratedParts(outputPathPrefix, isSplit)
	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("Đã tạo %d file/part nén tại '%s'", len(generatedParts), outputDir))
	for _, part := range generatedParts {
		logger.Debug(fmt.Sprintf("- Part: %s", part))
	}

	return generatedParts, nil
}

// compressFolderInternal (Giữ nguyên)
func compressFolderInternal(ctx context.Context, sourceDir, outputPath string, snapshotUsed bool) error {
	logSource := sourceDir
	if snapshotUsed {
		logSource = fmt.Sprintf("%s (từ snapshot)", sourceDir)
	}
	logger.Info(fmt.Sprintf("Chuẩn bị nén '%s' vào '%s'", logSource, outputPath))

	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("lỗi kiểm tra thư mục nguồn '%s': %w", sourceDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("đường dẫn nguồn '%s' không phải là thư mục", sourceDir)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("lỗi tạo thư mục output '%s': %w", outputDir, err)
	}

	if _, err := os.Stat(outputPath); err == nil {
		logger.Debug("Đang xóa file nén cũ:", outputPath)
		if errRem := os.Remove(outputPath); errRem != nil {
			logger.Warn("Không thể xóa file nén cũ:", errRem)
		}
	}

	profile, err := DetermineBestProfile()
	var compressionArgs []string
	if err != nil {
		logger.Warn("Không thể xác định cấu hình nén tốt nhất, dùng mặc định (-mx=5 -mmt=on):", err)
		compressionArgs = []string{"-mx=5", "-mmt=on"}
	} else {
		compressionArgs = profile.Arguments
		logger.Info("Sử dụng cấu hình nén được xác định tự động:", profile.Name, profile.Arguments)
	}

	args := []string{"a", "-y"}
	args = append(args, compressionArgs...)
	args = append(args, outputPath, sourceDir)

	return run7zCompress(ctx, args, fmt.Sprintf("CompressFolder for %s", sourceDir))
}

// --- Các hàm giải nén (Giữ nguyên) ---

func DecompressFolder(compressedFilePath, outputDir string) error {
	logger.Info(fmt.Sprintf("Giải nén archive '%s' vào thư mục '%s' bằng 7z", compressedFilePath, outputDir))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("lỗi khi tạo thư mục giải nén '%s': %w", outputDir, err)
	}
	if _, err := os.Stat(compressedFilePath); os.IsNotExist(err) {
		return fmt.Errorf("file nén nguồn '%s' không tồn tại", compressedFilePath)
	}
	args := []string{"x", compressedFilePath, "-o" + outputDir, "-y"}
	return run7zCompress(context.Background(), args, fmt.Sprintf("DecompressFolder for %s", compressedFilePath))
}

func DecompressFile(compressedFilePath, outputDir string) error {
	logger.Info(fmt.Sprintf("Giải nén archive '%s' (có thể là file đơn) vào thư mục '%s' bằng 7z", compressedFilePath, outputDir))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("lỗi khi tạo thư mục giải nén '%s': %w", outputDir, err)
	}
	if _, err := os.Stat(compressedFilePath); os.IsNotExist(err) {
		return fmt.Errorf("file nén nguồn '%s' không tồn tại", compressedFilePath)
	}
	args := []string{"x", compressedFilePath, "-o" + outputDir, "-y"}
	return run7zCompress(context.Background(), args, fmt.Sprintf("DecompressFile for %s", compressedFilePath))
}

// --- HÀM MỚI CÓ LOGIC SNAPSHOT ĐÃ SỬA ĐỔI ---

// replacePlaceholders thay thế các placeholder trong chuỗi lệnh mẫu.
func replacePlaceholders(template string, replacements map[string]string) string {
	command := template
	logger.Debug(fmt.Sprintf("--- replacePlaceholders --- START ---"))
	logger.Debug(fmt.Sprintf("  Template IN: [%s]", template))
	logger.Debug(fmt.Sprintf("  Replacements map: %v", replacements)) // Log the whole map

	for placeholder, value := range replacements {
		// Log before attempting replacement
		logger.Debug(fmt.Sprintf("  Attempting replace: placeholder='%s', value='%s'", placeholder, value))

		originalCommand := command // Store original for comparison
		command = strings.ReplaceAll(command, placeholder, value)

		// Log after replacement attempt and check if anything changed
		if command != originalCommand {
			logger.Debug(fmt.Sprintf("    REPLACED! command is now: [%s]", command))
		} else {
			logger.Debug(fmt.Sprintf("    NO CHANGE. Placeholder '%s' not found or value identical.", placeholder))
		}
	}
	logger.Debug(fmt.Sprintf("  Command OUT: [%s]", command))
	logger.Debug(fmt.Sprintf("--- replacePlaceholders --- END ---"))
	return command
}

// CompressFolderAndSplitWithOptionalSnapshot attempts to snapshot before compressing.
// Sử dụng tên snapshot duy nhất cho mỗi lần gọi.
func CompressFolderAndSplitWithOptionalSnapshot(ctx context.Context, sourceDir, outputDir, baseArchiveName string, splitSizeMB int) ([]string, error) {
	logger.Info("Gọi hàm nén (chia nhỏ) với tùy chọn snapshot...")

	// --- Đọc cấu hình Snapshot từ Environment Variables ---
	snapshotEnabled := strings.ToLower(os.Getenv("COMPRESS_ENABLE_SNAPSHOT")) == "true"
	createCmdTemplate := os.Getenv("COMPRESS_SNAPSHOT_CREATE_CMD")
	mountCmdTemplate := os.Getenv("COMPRESS_SNAPSHOT_MOUNT_CMD") // Mẫu lệnh mount
	mountPoint := os.Getenv("COMPRESS_SNAPSHOT_MOUNT_POINT")     // Đường dẫn mount CƠ SỞ (BASE)
	cleanupCmdTemplate := os.Getenv("COMPRESS_SNAPSHOT_CLEANUP_CMD")

	// Đọc thêm các biến cấu hình cần thiết cho placeholder
	snapshotNameBase := os.Getenv("SNAPSHOT_NAME_BASE") // Tiền tố tên snapshot
	vgName := os.Getenv("LVM_VG_NAME")
	lvPath := os.Getenv("LVM_LV_PATH")
	snapSize := os.Getenv("LVM_SNAPSHOT_SIZE")
	rsyncSrc := os.Getenv("RSYNC_MASTER_DATA_SRC_ABS") // Cho trường hợp Rsync

	currentSourceDir := sourceDir          // Mặc định nén từ thư mục gốc
	snapshotUsed := false                  // Cờ theo dõi việc sử dụng snapshot
	var cleanupCmdFinal string             // Lưu lệnh cleanup cuối cùng cho defer
	var snapshotCleanupNeeded bool = false // Cờ cho biết có cần chạy cleanup không

	if snapshotEnabled {
		logger.Info("Snapshot được bật qua biến môi trường.")

		// Kiểm tra các biến môi trường cần thiết
		if createCmdTemplate == "" || cleanupCmdTemplate == "" || snapshotNameBase == "" || mountPoint == "" {
			logger.Error("Snapshot được bật, nhưng thiếu cấu hình (CREATE_CMD, CLEANUP_CMD, SNAPSHOT_NAME_BASE, MOUNT_POINT). Bỏ qua snapshot.")
		} else {
			logger.Debug("(Snapshot) Đã đọc cấu hình snapshot từ env.") // **DEBUG LOG 1**
			// Tạo ID duy nhất cho lần gọi này
			uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
			snapshotNameUnique := snapshotNameBase + "_" + uniqueSuffix
			snapshotDeviceUnique := ""
			if vgName != "" {
				snapshotDeviceUnique = fmt.Sprintf("/dev/%s/%s", vgName, snapshotNameUnique)
			}

			// **DEBUG LOG 2**: Log giá trị mountPoint đọc từ env
			logger.Debug(fmt.Sprintf("(Snapshot) Giá trị COMPRESS_SNAPSHOT_MOUNT_POINT (mountPoint base): '%s'", mountPoint))
			if mountPoint == "" {
				logger.Error("Lỗi nghiêm trọng: COMPRESS_SNAPSHOT_MOUNT_POINT bị rỗng!")
				// Có thể trả lỗi ở đây nếu mountPoint base là bắt buộc
				// return nil, fmt.Errorf("biến môi trường COMPRESS_SNAPSHOT_MOUNT_POINT bị rỗng")
			}
			mountPointValue := os.Getenv("COMPRESS_SNAPSHOT_MOUNT_POINT") // Lấy giá trị từ biến môi trường

			// Chuẩn bị map thay thế cho các placeholder
			replacements := map[string]string{
				"$SNAPSHOT_NAME":             snapshotNameUnique,
				"$SNAPSHOT_DEVICE":           snapshotDeviceUnique,
				"$MOUNT_POINT":               mountPointValue,
				"$SNAPSHOT_MOUNT_POINT":      mountPoint, // Sử dụng BASE mount point trong replacements
				"$LVM_VG_NAME":               vgName,
				"$LVM_LV_PATH":               lvPath,
				"$LVM_SNAPSHOT_SIZE":         snapSize,
				"$RSYNC_MASTER_DATA_SRC_ABS": rsyncSrc,
			}
			logger.Debug("(Snapshot) Đã chuẩn bị map replacements.") // **DEBUG LOG 3**

			// Tạo các lệnh cuối cùng bằng cách thay thế placeholder
			createCmdFinal := replacePlaceholders(createCmdTemplate, replacements)
			mountCmdFinal := replacePlaceholders(mountCmdTemplate, replacements)
			cleanupCmdFinal = replacePlaceholders(cleanupCmdTemplate, replacements)
			logger.Debug("(Snapshot) Đã tạo các lệnh cuối cùng (create/mount/cleanup).") // **DEBUG LOG 4**

			// Defer cleanup sử dụng lệnh cleanupCmdFinal
			defer func() {
				logger.Debug("(Snapshot) Bắt đầu thực thi defer cleanup check...") // **DEBUG LOG (DEFER)**
				if snapshotCleanupNeeded {
					logger.Info("(Snapshot) Executing deferred cleanup command: ", cleanupCmdFinal)
					cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cancelCleanup()
					_, cleanupErr := executeExternalCommand(cleanupCtx, cleanupCmdFinal)
					if cleanupErr != nil {
						logger.Error("!!! LỖI CLEANUP SNAPSHOT !!! Cần cleanup thủ công. Lỗi:", cleanupErr)
					} else {
						logger.Info("(Snapshot) Thực thi lệnh cleanup thành công.")
					}
				} else {
					logger.Debug("(Snapshot) Bỏ qua cleanup vì snapshotCleanupNeeded=false.") // **DEBUG LOG (DEFER)**
				}
			}()

			// --- 1. Thực thi lệnh Tạo Snapshot/Rsync ---
			logger.Info("(Snapshot) Executing create command: ", createCmdFinal)
			_, createErr := executeExternalCommand(ctx, createCmdFinal)
			if createErr != nil {
				if ctx.Err() != nil {
					logger.Error("(Snapshot) Context bị hủy khi tạo snapshot.", ctx.Err())
					return nil, fmt.Errorf("context bị hủy khi tạo snapshot: %w", ctx.Err())
				}
				logger.Error("(Snapshot) Tạo snapshot/rsync thất bại, sẽ tiến hành nén thư mục gốc.", createErr)
				// Không đặt snapshotCleanupNeeded = true vì tạo đã lỗi
			} else {
				// Tạo thành công!
				logger.Info("(Snapshot) Thực thi lệnh tạo snapshot/rsync thành công.")
				snapshotCleanupNeeded = true // Đánh dấu cần cleanup sau này

				// *** LOGGING BỔ SUNG BẮT ĐẦU TỪ ĐÂY ***
				logger.Debug("(Snapshot) Đã thực thi xong Create. Bắt đầu logic Mount/Check...")                                                                                                            // **DEBUG LOG 5**
				logger.Debug(fmt.Sprintf("(Snapshot) Kiểm tra trước Mount: snapshotDeviceUnique='%s', mountCmdFinal (có thể rỗng)='%s', mountPoint='%s'", snapshotDeviceUnique, mountCmdFinal, mountPoint)) // **DEBUG LOG 6**

				// --- 2. Thực thi lệnh Mount Snapshot (Nếu có) hoặc kiểm tra ---
				snapshotPathToUse := ""
				// Chỉ mount/check nếu là LVM (có device) hoặc nếu là Rsync (có mount point)
				if snapshotDeviceUnique != "" { // Trường hợp LVM
					logger.Debug("(Snapshot) Trường hợp LVM (snapshotDeviceUnique không rỗng).") // **DEBUG LOG 7.LVM**
					if mountCmdFinal != "" {                                                     // Ưu tiên lệnh mount nếu có
						logger.Debug("(Snapshot) Có lệnh mount LVM được định nghĩa.") // **DEBUG LOG 8.LVM.MountCmd**
						// *** THÊM LOG NGAY TRƯỚC KHI GỌI EXECUTE MOUNT ***
						logger.Info("(Snapshot) Chuẩn bị thực thi mount command: ", mountCmdFinal) // **DEBUG LOG 9.LVM.MountCmd**
						_, mountErr := executeExternalCommand(ctx, mountCmdFinal)
						if mountErr != nil {
							if ctx.Err() != nil {
								logger.Error("(Snapshot) Context bị hủy khi mount snapshot.", ctx.Err())
								return nil, fmt.Errorf("context bị hủy khi mount snapshot: %w", ctx.Err()) // Defer cleanup sẽ chạy
							}
							logger.Error("(Snapshot) Mount snapshot thất bại.", mountErr)
							return nil, fmt.Errorf("snapshot đã tạo nhưng mount thất bại: %w", mountErr) // Defer cleanup sẽ chạy
						}
						logger.Info("(Snapshot) Thực thi lệnh mount thành công.")
						snapshotPathToUse = mountPoint // Sử dụng mount point đã cấu hình LÀM NGUỒN NÉN
					} else if mountPoint != "" {
						logger.Debug("(Snapshot) Không có lệnh mount LVM, kiểm tra sự tồn tại của mountPoint:", mountPoint) // **DEBUG LOG 8.LVM.NoMountCmd**
						if _, err := os.Stat(mountPoint); err == nil {
							logger.Info("(Snapshot) Sử dụng đường dẫn snapshot đã tồn tại (mount point LVM): ", mountPoint)
							snapshotPathToUse = mountPoint
						} else {
							logger.Error("(Snapshot) Mount point LVM được chỉ định nhưng không truy cập được (và không có lệnh mount):", mountPoint, err)
							return nil, fmt.Errorf("mount point snapshot LVM '%s' không truy cập được: %w", mountPoint, err) // Defer cleanup
						}
					} else {
						logger.Warn("(Snapshot) LVM: Không có lệnh mount và không có mount point được chỉ định. Không thể sử dụng snapshot.") // **DEBUG LOG 8.LVM.NoMountCmdNoMountPoint**
					}
				} else if mountPoint != "" { // Trường hợp Rsync (không có device, nhưng có mount point)
					logger.Debug("(Snapshot) Trường hợp Rsync (snapshotDeviceUnique rỗng, mountPoint không rỗng). Kiểm tra mountPoint:", mountPoint) // **DEBUG LOG 7.Rsync**
					if _, err := os.Stat(mountPoint); err == nil {
						logger.Info("(Snapshot) Sử dụng đường dẫn snapshot đã tồn tại (rsync target): ", mountPoint)
						snapshotPathToUse = mountPoint
					} else {
						logger.Error("(Snapshot) Mount point (rsync target) được chỉ định nhưng không truy cập được:", mountPoint, err)
						return nil, fmt.Errorf("rsync target '%s' không truy cập được: %w", mountPoint, err) // Defer cleanup (ví dụ: xóa thư mục rsync nếu nó tồn tại)
					}
				} else {
					// Trường hợp không phải LVM và cũng không có mount point (lạ?)
					logger.Warn("(Snapshot) Không phải LVM và không có mount point. Không thể sử dụng snapshot.") // **DEBUG LOG 7.Else**
				}

				// --- 3. Xác định thư mục nguồn cuối cùng để nén ---
				logger.Debug(fmt.Sprintf("(Snapshot) Sau khi mount/check, snapshotPathToUse = '%s'", snapshotPathToUse)) // **DEBUG LOG 10**
				if snapshotPathToUse != "" {
					// Kiểm tra lại xem có phải thư mục không
					logger.Debug("(Snapshot) Kiểm tra Stat của snapshotPathToUse:", snapshotPathToUse) // **DEBUG LOG 11**
					info, err := os.Stat(snapshotPathToUse)
					if err != nil {
						logger.Error("(Snapshot) Không thể truy cập đường dẫn snapshot sau khi mount/tạo:", snapshotPathToUse, err)
						return nil, fmt.Errorf("không thể truy cập snapshot tại '%s': %w", snapshotPathToUse, err) // Defer cleanup sẽ chạy
					}
					if !info.IsDir() {
						logger.Error("(Snapshot) Đường dẫn snapshot không phải là thư mục:", snapshotPathToUse)
						return nil, fmt.Errorf("đường dẫn snapshot '%s' không phải là thư mục", snapshotPathToUse) // Defer cleanup sẽ chạy
					}
					currentSourceDir = snapshotPathToUse // Cập nhật để nén từ snapshot
					snapshotUsed = true
					logger.Info("(Snapshot) Nén sẽ được thực hiện trên:", currentSourceDir)
				} else {
					logger.Warn("(Snapshot) Không xác định được đường dẫn snapshot để sử dụng sau khi tạo. Sẽ nén thư mục gốc.")
					// snapshotCleanupNeeded vẫn là true để xóa snapshot/rsync đã tạo (nếu có).
				}
			} // Kết thúc của else (createErr == nil)
		} // Kết thúc của else (kiểm tra biến môi trường)
	} else { // snapshotEnabled == false
		logger.Info("Snapshot không được bật. Sẽ nén thư mục gốc.")
	} // Kết thúc của if snapshotEnabled

	logger.Debug(fmt.Sprintf("(Snapshot) Chuẩn bị gọi compressFolderAndSplitInternal với currentSourceDir='%s', snapshotUsed=%t", currentSourceDir, snapshotUsed)) // **DEBUG LOG 12**
	// Gọi hàm nén nội bộ với sourceDir đã được xác định (gốc hoặc snapshot)
	// và truyền context gốc vào
	return compressFolderAndSplitInternal(ctx, currentSourceDir, outputDir, baseArchiveName, splitSizeMB, snapshotUsed)
}

// CompressFolderWithOptionalSnapshot attempts to snapshot before compressing a folder to a single archive.
// Tương tự như hàm split nhưng không có logic splitSizeMB.
func CompressFolderWithOptionalSnapshot(ctx context.Context, sourceDir, outputPath string) error {
	logger.Info("Gọi hàm nén thư mục (file đơn) với tùy chọn snapshot...")

	// --- Logic đọc cấu hình và tạo tên/lệnh duy nhất (tương tự như hàm split) ---
	snapshotEnabled := strings.ToLower(os.Getenv("COMPRESS_ENABLE_SNAPSHOT")) == "true"
	createCmdTemplate := os.Getenv("COMPRESS_SNAPSHOT_CREATE_CMD")
	mountCmdTemplate := os.Getenv("COMPRESS_SNAPSHOT_MOUNT_CMD")
	mountPoint := os.Getenv("COMPRESS_SNAPSHOT_MOUNT_POINT")
	cleanupCmdTemplate := os.Getenv("COMPRESS_SNAPSHOT_CLEANUP_CMD")

	snapshotNameBase := os.Getenv("SNAPSHOT_NAME_BASE")
	vgName := os.Getenv("LVM_VG_NAME")
	lvPath := os.Getenv("LVM_LV_PATH")
	snapSize := os.Getenv("LVM_SNAPSHOT_SIZE")
	rsyncSrc := os.Getenv("RSYNC_MASTER_DATA_SRC_ABS")

	currentSourceDir := sourceDir
	snapshotUsed := false
	var cleanupCmdFinal string
	var snapshotCleanupNeeded bool = false

	if snapshotEnabled {
		logger.Info("Snapshot được bật qua biến môi trường.")
		if createCmdTemplate == "" || cleanupCmdTemplate == "" || snapshotNameBase == "" || mountPoint == "" {
			logger.Error("Snapshot được bật, nhưng thiếu cấu hình. Bỏ qua snapshot.")
		} else {
			uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
			snapshotNameUnique := snapshotNameBase + "_" + uniqueSuffix
			snapshotDeviceUnique := ""
			if vgName != "" {
				snapshotDeviceUnique = fmt.Sprintf("/dev/%s/%s", vgName, snapshotNameUnique)
			}
			mountPointValue := os.Getenv("COMPRESS_SNAPSHOT_MOUNT_POINT") // Lấy giá trị từ biến môi trường

			replacements := map[string]string{
				"$SNAPSHOT_NAME":             snapshotNameUnique,
				"$SNAPSHOT_DEVICE":           snapshotDeviceUnique,
				"$SNAPSHOT_MOUNT_POINT":      mountPoint,
				"$MOUNT_POINT":               mountPointValue,
				"$LVM_VG_NAME":               vgName,
				"$LVM_LV_PATH":               lvPath,
				"$LVM_SNAPSHOT_SIZE":         snapSize,
				"$RSYNC_MASTER_DATA_SRC_ABS": rsyncSrc,
			}

			createCmdFinal := replacePlaceholders(createCmdTemplate, replacements)
			mountCmdFinal := replacePlaceholders(mountCmdTemplate, replacements)
			cleanupCmdFinal = replacePlaceholders(cleanupCmdTemplate, replacements)

			defer func() {
				if snapshotCleanupNeeded {
					logger.Info("(Snapshot) Executing deferred cleanup command: ", cleanupCmdFinal)
					cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cancelCleanup()
					_, cleanupErr := executeExternalCommand(cleanupCtx, cleanupCmdFinal)
					if cleanupErr != nil {
						logger.Error("!!! LỖI CLEANUP SNAPSHOT !!! Cần cleanup thủ công. Lỗi:", cleanupErr)
					} else {
						logger.Info("(Snapshot) Thực thi lệnh cleanup thành công.")
					}
				}
			}()

			logger.Info("(Snapshot) Executing create command: ", createCmdFinal)
			_, createErr := executeExternalCommand(ctx, createCmdFinal)
			if createErr != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("context bị hủy khi tạo snapshot: %w", ctx.Err())
				}
				logger.Error("(Snapshot) Tạo snapshot/rsync thất bại, sẽ tiến hành nén thư mục gốc.", createErr)
			} else {
				logger.Info("(Snapshot) Thực thi lệnh tạo snapshot/rsync thành công.")
				snapshotCleanupNeeded = true

				snapshotPathToUse := ""
				if snapshotDeviceUnique != "" { // LVM Case
					if mountCmdFinal != "" {
						logger.Info("(Snapshot) Executing mount command: ", mountCmdFinal)
						_, mountErr := executeExternalCommand(ctx, mountCmdFinal)
						if mountErr != nil {
							if ctx.Err() != nil {
								return fmt.Errorf("context bị hủy khi mount snapshot: %w", ctx.Err())
							}
							logger.Error("(Snapshot) Mount snapshot thất bại.", mountErr)
							return fmt.Errorf("snapshot đã tạo nhưng mount thất bại: %w", mountErr) // Defer cleanup will run
						}
						logger.Info("(Snapshot) Thực thi lệnh mount thành công.")
						snapshotPathToUse = mountPoint
					} else if mountPoint != "" {
						if _, err := os.Stat(mountPoint); err == nil {
							logger.Info("(Snapshot) Sử dụng đường dẫn snapshot đã tồn tại (mount point): ", mountPoint)
							snapshotPathToUse = mountPoint
						} else {
							logger.Error("(Snapshot) Mount point được chỉ định nhưng không truy cập được (và không có lệnh mount):", mountPoint, err)
							return fmt.Errorf("mount point snapshot '%s' không truy cập được và không có lệnh mount: %w", mountPoint, err) // Defer cleanup
						}
					}
				} else if mountPoint != "" { // Rsync Case
					if _, err := os.Stat(mountPoint); err == nil {
						logger.Info("(Snapshot) Sử dụng đường dẫn snapshot đã tồn tại (rsync target): ", mountPoint)
						snapshotPathToUse = mountPoint
					} else {
						logger.Error("(Snapshot) Mount point (rsync target) được chỉ định nhưng không truy cập được:", mountPoint, err)
						return fmt.Errorf("rsync target '%s' không truy cập được: %w", mountPoint, err) // Defer cleanup
					}
				}

				if snapshotPathToUse != "" {
					info, err := os.Stat(snapshotPathToUse)
					if err != nil {
						logger.Error("(Snapshot) Không thể truy cập đường dẫn snapshot sau khi mount/tạo:", snapshotPathToUse, err)
						return fmt.Errorf("không thể truy cập snapshot tại '%s': %w", snapshotPathToUse, err) // Defer cleanup
					}
					if !info.IsDir() {
						logger.Error("(Snapshot) Đường dẫn snapshot không phải là thư mục:", snapshotPathToUse)
						return fmt.Errorf("đường dẫn snapshot '%s' không phải là thư mục", snapshotPathToUse) // Defer cleanup
					}
					currentSourceDir = snapshotPathToUse
					snapshotUsed = true
					logger.Info("(Snapshot) Nén sẽ được thực hiện trên:", currentSourceDir)
				} else {
					logger.Warn("(Snapshot) Không xác định được đường dẫn snapshot để sử dụng. Sẽ nén thư mục gốc.")
				}
			}
		}
	} else {
		logger.Info("Snapshot không được bật. Sẽ nén thư mục gốc.")
	}

	// Gọi hàm nén nội bộ compressFolderInternal với context được truyền vào
	return compressFolderInternal(ctx, currentSourceDir, outputPath, snapshotUsed)
}
