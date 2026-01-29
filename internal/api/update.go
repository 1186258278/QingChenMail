package api

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"goemail/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/minio/selfupdate"
)

// GitHub Release 结构
type GitHubRelease struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	PublishedAt string         `json:"published_at"`
	Assets      []GitHubAsset  `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateInfo 更新信息
type UpdateInfo struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseNotes   string `json:"release_notes"`
	PublishedAt    string `json:"published_at"`
	DownloadURL    string `json:"download_url"`
	DownloadSize   int64  `json:"download_size"`
	FileName       string `json:"file_name"`
	Platform       string `json:"platform"`
}

// UpdateStatus 更新状态
type UpdateStatus struct {
	Status        string `json:"status"` // idle, checking, downloading, extracting, applying, completed, failed
	Progress      int    `json:"progress"`
	Message       string `json:"message"`
	Error         string `json:"error,omitempty"`
	NeedsRestart  bool   `json:"needs_restart"`
}

var (
	updateMutex   sync.Mutex
	currentStatus = UpdateStatus{Status: "idle", Progress: 0, Message: "就绪"}
)

// 全局版本缓存 - 用于快速返回版本检测结果
var (
	cachedUpdateInfo  *UpdateInfo
	cachedUpdateTime  time.Time
	cachedUpdateMutex sync.RWMutex
)

// updateCache 更新版本缓存
func updateCache(info *UpdateInfo) {
	cachedUpdateMutex.Lock()
	defer cachedUpdateMutex.Unlock()
	cachedUpdateInfo = info
	cachedUpdateTime = time.Now()
}

// GetCachedUpdateHandler 获取缓存的版本信息（快速响应，不触发 GitHub 请求）
func GetCachedUpdateHandler(c *gin.Context) {
	cachedUpdateMutex.RLock()
	defer cachedUpdateMutex.RUnlock()

	if cachedUpdateInfo == nil {
		// 缓存为空，返回基本信息
		c.JSON(http.StatusOK, gin.H{
			"has_update":      false,
			"current_version": config.Version,
			"cached":          false,
			"message":         "缓存未初始化，请等待后台检测或手动检测",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_update":      cachedUpdateInfo.HasUpdate,
		"current_version": cachedUpdateInfo.CurrentVersion,
		"latest_version":  cachedUpdateInfo.LatestVersion,
		"release_notes":   cachedUpdateInfo.ReleaseNotes,
		"published_at":    cachedUpdateInfo.PublishedAt,
		"download_url":    cachedUpdateInfo.DownloadURL,
		"cached":          true,
		"cached_at":       cachedUpdateTime.Format(time.RFC3339),
	})
}

// GetUpdateInfoHandler 获取更新信息 (增强版)
func GetUpdateInfoHandler(c *gin.Context) {
	updateMutex.Lock()
	currentStatus = UpdateStatus{Status: "checking", Progress: 0, Message: "正在检查更新..."}
	updateMutex.Unlock()

	// 获取 GitHub Release 信息
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/1186258278/QingChenMail/releases/latest")
	if err != nil {
		setStatusError("无法连接到 GitHub: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法连接到更新服务器"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		setStatusError("GitHub API 返回错误")
		c.JSON(resp.StatusCode, gin.H{"error": "GitHub API 错误"})
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		setStatusError("解析版本信息失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析版本信息失败"})
		return
	}

	// 当前版本
	currentVer := config.Version

	// 检查是否有更新
	hasUpdate := compareVersions(release.TagName, currentVer) > 0

	// 查找适合当前平台的下载包
	downloadURL, fileName, fileSize := findPlatformAsset(release.Assets)

	info := UpdateInfo{
		HasUpdate:      hasUpdate,
		CurrentVersion: currentVer,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		PublishedAt:    release.PublishedAt,
		DownloadURL:    downloadURL,
		DownloadSize:   fileSize,
		FileName:       fileName,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	// 更新全局缓存
	updateCache(&info)

	updateMutex.Lock()
	currentStatus = UpdateStatus{Status: "idle", Progress: 100, Message: "检查完成"}
	updateMutex.Unlock()

	c.JSON(http.StatusOK, info)
}

// PerformUpdateHandler 执行在线更新
func PerformUpdateHandler(c *gin.Context) {
	updateMutex.Lock()
	if currentStatus.Status == "downloading" || currentStatus.Status == "applying" {
		updateMutex.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "更新正在进行中"})
		return
	}
	currentStatus = UpdateStatus{Status: "downloading", Progress: 0, Message: "开始下载..."}
	updateMutex.Unlock()

	// 获取请求参数
	var req struct {
		DownloadURL string `json:"download_url"`
		FileName    string `json:"file_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DownloadURL == "" {
		setStatusError("无效的更新请求")
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的更新请求"})
		return
	}

	// 异步执行更新
	go func() {
		if err := doUpdate(req.DownloadURL, req.FileName); err != nil {
			setStatusError(err.Error())
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "更新已开始", "status": "downloading"})
}

// GetUpdateStatusHandler 获取更新状态
func GetUpdateStatusHandler(c *gin.Context) {
	updateMutex.Lock()
	status := currentStatus
	updateMutex.Unlock()
	c.JSON(http.StatusOK, status)
}

// doUpdate 执行实际更新逻辑
func doUpdate(downloadURL, fileName string) error {
	// 0. 创建备份
	setStatus("backing_up", 5, "正在创建备份...")
	backupID, err := CreateBackup(config.Version, true)
	if err != nil {
		return fmt.Errorf("创建备份失败: %w", err)
	}
	setStatus("backing_up", 8, fmt.Sprintf("备份完成: %s", backupID))

	// 1. 下载文件
	setStatus("downloading", 10, "正在下载更新包...")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "goemail-update-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 保存到临时文件
	tempArchive := filepath.Join(tempDir, fileName)
	outFile, err := os.Create(tempArchive)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 带进度的下载
	totalSize := resp.ContentLength
	downloaded := int64(0)
	reader := &progressReader{
		reader: resp.Body,
		onProgress: func(n int64) {
			downloaded += n
			if totalSize > 0 {
				progress := int(float64(downloaded) / float64(totalSize) * 50) + 10 // 10-60%
				setStatus("downloading", progress, fmt.Sprintf("正在下载... %.1f%%", float64(downloaded)/float64(totalSize)*100))
			}
		},
	}

	if _, err := io.Copy(outFile, reader); err != nil {
		outFile.Close()
		return fmt.Errorf("下载写入失败: %w", err)
	}
	outFile.Close()

	setStatus("extracting", 60, "正在解压更新包...")

	// 2. 解压文件
	var binaryPath string
	if strings.HasSuffix(fileName, ".tar.gz") {
		binaryPath, err = extractTarGz(tempArchive, tempDir)
	} else if strings.HasSuffix(fileName, ".zip") {
		binaryPath, err = extractZip(tempArchive, tempDir)
	} else {
		return fmt.Errorf("不支持的压缩格式: %s", fileName)
	}

	if err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}

	if binaryPath == "" {
		return fmt.Errorf("未找到可执行文件")
	}

	setStatus("applying", 80, "正在应用更新...")

	// 3. 获取当前可执行文件路径
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %w", err)
	}
	
	// 尝试解析符号链接，如果失败则使用原路径
	if resolved, err := filepath.EvalSymlinks(currentExe); err == nil {
		currentExe = resolved
	}
	// 清理绝对路径
	currentExe, _ = filepath.Abs(currentExe)

	// 4. 清理可能存在的旧 .old 文件（selfupdate 遗留）
	oldFile := currentExe + ".old"
	if _, err := os.Stat(oldFile); err == nil {
		os.Remove(oldFile)
		fmt.Printf("[Update] 已清理旧备份文件: %s\n", oldFile)
	}

	// 5. 备份当前版本
	backupPath := currentExe + ".backup"
	setStatus("applying", 85, "正在备份当前版本...")
	
	// 读取当前文件用于备份
	currentData, err := os.ReadFile(currentExe)
	if err != nil {
		return fmt.Errorf("读取当前程序失败: %w", err)
	}
	if err := os.WriteFile(backupPath, currentData, 0755); err != nil {
		return fmt.Errorf("备份失败: %w", err)
	}

	// 6. 使用 selfupdate 应用更新
	setStatus("applying", 90, "正在替换程序文件...")

	newBinary, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("打开新版本文件失败: %w", err)
	}
	defer newBinary.Close()

	// 指定目标路径，避免 selfupdate 自动检测出错
	err = selfupdate.Apply(newBinary, selfupdate.Options{
		TargetPath: currentExe,
	})
	if err != nil {
		// 回滚
		if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
			return fmt.Errorf("更新失败且回滚失败: %w, rollback: %v", err, rollbackErr)
		}
		return fmt.Errorf("更新失败 (已回滚): %w", err)
	}

	setStatus("completed", 100, "更新完成！请重启服务以应用新版本。")
	updateMutex.Lock()
	currentStatus.NeedsRestart = true
	updateMutex.Unlock()

	return nil
}

// progressReader 带进度回调的 Reader
type progressReader struct {
	reader     io.Reader
	onProgress func(int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.onProgress != nil {
		pr.onProgress(int64(n))
	}
	return n, err
}

// extractTarGz 解压 tar.gz 文件
func extractTarGz(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var binaryPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			// 查找 goemail 可执行文件
			baseName := filepath.Base(header.Name)
			if baseName == "goemail" || baseName == "goemail.exe" {
				outFile, err := os.Create(target)
				if err != nil {
					return "", err
				}
				if _, err := io.Copy(outFile, tr); err != nil {
					outFile.Close()
					return "", err
				}
				outFile.Close()
				os.Chmod(target, 0755)
				binaryPath = target
			}
		}
	}

	return binaryPath, nil
}

// extractZip 解压 zip 文件
func extractZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var binaryPath string

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		// 查找 goemail 可执行文件
		baseName := filepath.Base(f.Name)
		if baseName == "goemail" || baseName == "goemail.exe" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			outFile, err := os.Create(target)
			if err != nil {
				rc.Close()
				return "", err
			}

			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()

			if err != nil {
				return "", err
			}
			os.Chmod(target, 0755)
			binaryPath = target
		}
	}

	return binaryPath, nil
}

// findPlatformAsset 查找当前平台对应的下载包
func findPlatformAsset(assets []GitHubAsset) (downloadURL, fileName string, size int64) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// 构建期望的文件名模式
	var expectedPatterns []string
	switch goos {
	case "linux":
		if goarch == "amd64" {
			expectedPatterns = []string{"Linux_x86_64.tar.gz", "linux_amd64.tar.gz"}
		} else if goarch == "arm64" {
			expectedPatterns = []string{"Linux_arm64.tar.gz", "linux_arm64.tar.gz"}
		}
	case "darwin":
		if goarch == "amd64" {
			expectedPatterns = []string{"Darwin_x86_64.tar.gz", "darwin_amd64.tar.gz"}
		} else if goarch == "arm64" {
			expectedPatterns = []string{"Darwin_arm64.tar.gz", "darwin_arm64.tar.gz"}
		}
	case "windows":
		if goarch == "amd64" {
			expectedPatterns = []string{"Windows_x86_64.zip", "windows_amd64.zip"}
		}
	}

	for _, asset := range assets {
		for _, pattern := range expectedPatterns {
			if strings.Contains(asset.Name, pattern) || strings.HasSuffix(asset.Name, pattern) {
				return asset.BrowserDownloadURL, asset.Name, asset.Size
			}
		}
	}

	// 如果没有精确匹配，尝试模糊匹配
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, goos) && strings.Contains(name, goarch) {
			return asset.BrowserDownloadURL, asset.Name, asset.Size
		}
		// x86_64 等同于 amd64
		if goarch == "amd64" && strings.Contains(name, goos) && strings.Contains(name, "x86_64") {
			return asset.BrowserDownloadURL, asset.Name, asset.Size
		}
	}

	return "", "", 0
}

// compareVersions 比较版本号 (返回: 1 if v1 > v2, -1 if v1 < v2, 0 if equal)
func compareVersions(v1, v2 string) int {
	// 移除 'v' 前缀
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	return 0
}

// setStatus 设置更新状态
func setStatus(status string, progress int, message string) {
	updateMutex.Lock()
	currentStatus.Status = status
	currentStatus.Progress = progress
	currentStatus.Message = message
	updateMutex.Unlock()
}

// setStatusError 设置错误状态
func setStatusError(errMsg string) {
	updateMutex.Lock()
	currentStatus.Status = "failed"
	currentStatus.Error = errMsg
	currentStatus.Message = "更新失败"
	updateMutex.Unlock()
}

// GetChecksumsHandler 获取并验证 checksum (可选，用于高级验证)
func GetChecksumsHandler(c *gin.Context) {
	version := c.Query("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少版本参数"})
		return
	}

	// 下载 checksums.txt
	url := fmt.Sprintf("https://github.com/1186258278/QingChenMail/releases/download/%s/checksums.txt", version)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取校验文件"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "校验文件不存在"})
		return
	}

	// 解析 checksums
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 {
			checksums[parts[1]] = parts[0]
		}
	}

	c.JSON(http.StatusOK, checksums)
}

// verifyChecksum 验证文件校验和
func verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("校验和不匹配: 期望 %s, 实际 %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// RestartHandler 重启服务
func RestartHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "正在重启服务..."})

	// 延迟执行重启，让响应先返回
	go func() {
		time.Sleep(500 * time.Millisecond)
		RestartSelf()
	}()
}

// RestartSelf 重启当前程序
func RestartSelf() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("获取程序路径失败: %v\n", err)
		return
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Printf("解析程序路径失败: %v\n", err)
		return
	}

	// 检查是否有待处理的更新
	pendingUpdate := exe + ".pending"
	if _, err := os.Stat(pendingUpdate); err == nil {
		// 有待处理的更新，先应用
		if err := os.Rename(pendingUpdate, exe); err != nil {
			fmt.Printf("应用待处理更新失败: %v\n", err)
		}
	}

	fmt.Println("[Update] 正在重启服务...")

	if runtime.GOOS == "windows" {
		// Windows: 使用 cmd 启动新进程
		// 创建一个批处理脚本来延迟启动
		restartScript := filepath.Join(os.TempDir(), "goemail-restart.bat")
		script := fmt.Sprintf(`@echo off
ping 127.0.0.1 -n 2 > nul
start "" "%s"
del "%%~f0"
`, exe)
		if err := os.WriteFile(restartScript, []byte(script), 0755); err != nil {
			fmt.Printf("创建重启脚本失败: %v\n", err)
			return
		}

		cmd := exec.Command("cmd", "/C", restartScript)
		cmd.Start()
		os.Exit(0)
	} else {
		// Linux/macOS: 使用 syscall.Exec 替换当前进程
		args := os.Args
		env := os.Environ()

		fmt.Printf("[Update] 执行: %s %v\n", exe, args)

		// 使用 syscall.Exec 直接替换进程
		err := syscall.Exec(exe, args, env)
		if err != nil {
			fmt.Printf("重启失败: %v\n", err)
			// 如果 syscall.Exec 失败，尝试使用 exec.Command
			cmd := exec.Command(exe, args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Start()
			os.Exit(0)
		}
	}
}

// ============================================
// 自动更新检测
// ============================================

var autoUpdateRunning bool
var versionCacheRunning bool

// StartVersionCacheUpdater 启动版本缓存更新后台任务（每60分钟检测一次）
func StartVersionCacheUpdater() {
	if versionCacheRunning {
		return
	}
	versionCacheRunning = true

	go func() {
		// 启动时立即检测一次，填充缓存
		fmt.Println("[VersionCache] 正在初始化版本缓存...")
		if info, err := checkForUpdateInternal(); err == nil {
			updateCache(info)
			fmt.Printf("[VersionCache] 缓存已初始化: 当前 %s, 最新 %s\n", info.CurrentVersion, info.LatestVersion)
		} else {
			fmt.Printf("[VersionCache] 初始化失败: %v\n", err)
		}

		// 每 60 分钟检测一次
		ticker := time.NewTicker(60 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			fmt.Println("[VersionCache] 定时检测版本...")
			if info, err := checkForUpdateInternal(); err == nil {
				updateCache(info)
				if info.HasUpdate {
					fmt.Printf("[VersionCache] 发现新版本: %s -> %s\n", info.CurrentVersion, info.LatestVersion)
				}
			} else {
				fmt.Printf("[VersionCache] 检测失败: %v\n", err)
			}
		}
	}()

	fmt.Println("[VersionCache] 版本缓存更新任务已启动（每60分钟检测）")
}

// StartAutoUpdateChecker 启动自动更新检测后台任务
func StartAutoUpdateChecker() {
	if autoUpdateRunning {
		return
	}
	autoUpdateRunning = true

	go func() {
		// 启动时等待 1 分钟，让服务完全启动
		time.Sleep(1 * time.Minute)

		for {
			// 检查是否启用自动更新
			if !config.AppConfig.AutoUpdateEnabled {
				time.Sleep(1 * time.Hour) // 即使关闭也定期检查配置变化
				continue
			}

			// 获取检查间隔
			interval := config.AppConfig.AutoUpdateInterval
			if interval <= 0 {
				interval = 24 // 默认 24 小时
			}

			// 检查是否到达更新时间
			if isAutoUpdateTime() {
				fmt.Println("[AutoUpdate] 检查更新...")
				
				// 检查更新
				info, err := checkForUpdateInternal()
				if err != nil {
					fmt.Printf("[AutoUpdate] 检查更新失败: %v\n", err)
				} else {
					// 同步更新缓存
					updateCache(info)
					
					if info.HasUpdate {
						fmt.Printf("[AutoUpdate] 发现新版本: %s -> %s\n", info.CurrentVersion, info.LatestVersion)
						
						// 执行自动更新
						if err := doUpdate(info.DownloadURL, info.FileName); err != nil {
							fmt.Printf("[AutoUpdate] 自动更新失败: %v\n", err)
						} else {
							fmt.Println("[AutoUpdate] 更新成功，正在重启...")
							RestartSelf()
						}
					} else {
						fmt.Println("[AutoUpdate] 当前已是最新版本")
					}
				}
			}

			time.Sleep(time.Duration(interval) * time.Hour)
		}
	}()

	fmt.Println("[AutoUpdate] 自动更新检测已启动")
}

// isAutoUpdateTime 检查是否到达自动更新时间
func isAutoUpdateTime() bool {
	updateTime := config.AppConfig.AutoUpdateTime
	if updateTime == "" {
		updateTime = "03:00" // 默认凌晨 3 点
	}

	// 解析配置的时间
	parts := strings.Split(updateTime, ":")
	if len(parts) != 2 {
		return false
	}

	var configHour, configMin int
	fmt.Sscanf(parts[0], "%d", &configHour)
	fmt.Sscanf(parts[1], "%d", &configMin)

	now := time.Now()
	// 检查当前时间是否在配置时间的前后 30 分钟内
	configTime := time.Date(now.Year(), now.Month(), now.Day(), configHour, configMin, 0, 0, now.Location())
	diff := now.Sub(configTime)
	
	return diff >= 0 && diff < 30*time.Minute
}

// checkForUpdateInternal 内部使用的更新检查函数
func checkForUpdateInternal() (*UpdateInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/1186258278/QingChenMail/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 错误: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	currentVer := config.Version
	hasUpdate := compareVersions(release.TagName, currentVer) > 0
	downloadURL, fileName, fileSize := findPlatformAsset(release.Assets)

	return &UpdateInfo{
		HasUpdate:      hasUpdate,
		CurrentVersion: currentVer,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		PublishedAt:    release.PublishedAt,
		DownloadURL:    downloadURL,
		DownloadSize:   fileSize,
		FileName:       fileName,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}, nil
}

// GetAutoUpdateConfigHandler 获取自动更新配置
func GetAutoUpdateConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled":  config.AppConfig.AutoUpdateEnabled,
		"interval": config.AppConfig.AutoUpdateInterval,
		"time":     config.AppConfig.AutoUpdateTime,
	})
}

// UpdateAutoUpdateConfigHandler 更新自动更新配置
func UpdateAutoUpdateConfigHandler(c *gin.Context) {
	var req struct {
		Enabled  bool   `json:"enabled"`
		Interval int    `json:"interval"`
		Time     string `json:"time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 更新配置
	config.AppConfig.AutoUpdateEnabled = req.Enabled
	config.AppConfig.AutoUpdateInterval = req.Interval
	config.AppConfig.AutoUpdateTime = req.Time

	// 保存配置
	config.SaveConfig(config.AppConfig)

	c.JSON(http.StatusOK, gin.H{
		"message":  "自动更新配置已保存",
		"enabled":  config.AppConfig.AutoUpdateEnabled,
		"interval": config.AppConfig.AutoUpdateInterval,
		"time":     config.AppConfig.AutoUpdateTime,
	})
}
