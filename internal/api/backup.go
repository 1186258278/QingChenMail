package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"goemail/internal/config"
	"goemail/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// BackupInfo 备份信息结构
type BackupInfo struct {
	ID        string    `json:"id"`         // backup-v1.1.0-20260130-120000
	Version   string    `json:"version"`    // v1.1.0
	CreatedAt time.Time `json:"created_at"` // 创建时间
	Size      int64     `json:"size"`       // 总大小（字节）
	IsAuto    bool      `json:"is_auto"`    // 是否自动备份
	Files     []string  `json:"files"`      // 包含的文件列表
}

// BackupManifest 备份清单
type BackupManifest struct {
	ID        string   `json:"id"`
	Version   string   `json:"version"`
	CreatedAt string   `json:"created_at"`
	IsAuto    bool     `json:"is_auto"`
	Files     []string `json:"files"`
}

const backupDir = "backups"

// ListBackupsHandler 获取备份列表
func ListBackupsHandler(c *gin.Context) {
	backups, err := listBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取备份列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, backups)
}

// CreateBackupHandler 手动创建备份
func CreateBackupHandler(c *gin.Context) {
	backupID, err := CreateBackup(config.Version, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建备份失败: " + err.Error()})
		return
	}

	// 获取新创建的备份信息
	backups, _ := listBackups()
	for _, b := range backups {
		if b.ID == backupID {
			c.JSON(http.StatusOK, gin.H{
				"message": "备份创建成功 (备份不含发送日志/转发日志，以减小体积)",
				"backup":  b,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "备份创建成功 (备份不含发送日志/转发日志，以减小体积)",
		"backup_id": backupID,
	})
}

// validateBackupID 校验备份 ID 格式，防止路径遍历 (如 "."、"../backups2" 等)
// 合法格式: backup-<version>-<timestamp>，只允许字母数字和 . _ -
func validateBackupID(backupID string) bool {
	if backupID == "" || backupID == "." || backupID == ".." {
		return false
	}
	if strings.ContainsAny(backupID, "/\\") {
		return false
	}
	for _, r := range backupID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// RestoreBackupHandler 恢复到指定备份
func RestoreBackupHandler(c *gin.Context) {
	backupID := c.Param("id")
	if backupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "备份 ID 不能为空"})
		return
	}
	if !validateBackupID(backupID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份 ID"})
		return
	}

	// 路径遍历防护：确保路径在备份目录内
	backupPath := filepath.Join(backupDir, backupID)
	absPath, err := filepath.Abs(backupPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份路径"})
		return
	}
	absBackupDir, _ := filepath.Abs(backupDir)
	if absPath != absBackupDir && !strings.HasPrefix(absPath, absBackupDir+string(os.PathSeparator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份路径"})
		return
	}

	// 验证备份是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份不存在"})
		return
	}

	// 读取清单
	manifestPath := filepath.Join(backupPath, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取备份清单失败"})
		return
	}

	var manifest BackupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析备份清单失败"})
		return
	}

	// 恢复文件
	restoredFiles := []string{}

	// 恢复数据库
	dbBackup := filepath.Join(backupPath, "goemail.db")
	if _, err := os.Stat(dbBackup); err == nil {
		if err := copyFile(dbBackup, "goemail.db"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复数据库失败: " + err.Error()})
			return
		}
		restoredFiles = append(restoredFiles, "goemail.db")
	}

	// 恢复配置文件
	configBackup := filepath.Join(backupPath, "config.json")
	if _, err := os.Stat(configBackup); err == nil {
		if err := copyFile(configBackup, "config.json"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复配置文件失败: " + err.Error()})
			return
		}
		restoredFiles = append(restoredFiles, "config.json")
		// 重新加载配置
		config.LoadConfig()
	}

	// 恢复程序文件（标记为需要重启）
	exeBackup := filepath.Join(backupPath, "goemail.backup")
	needsRestart := false
	if _, err := os.Stat(exeBackup); err == nil {
		currentExe, err := os.Executable()
		if err == nil {
			currentExe, _ = filepath.EvalSymlinks(currentExe)
			// 复制到临时位置，重启时替换
			pendingUpdate := currentExe + ".pending"
			if err := copyFile(exeBackup, pendingUpdate); err == nil {
				restoredFiles = append(restoredFiles, filepath.Base(currentExe))
				needsRestart = true
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "恢复成功 (注意: 备份不含发送日志/转发日志，恢复后这些数据为空)",
		"restored":      restoredFiles,
		"needs_restart": needsRestart,
		"version":       manifest.Version,
	})
}

// DeleteBackupHandler 删除指定备份
func DeleteBackupHandler(c *gin.Context) {
	backupID := c.Param("id")
	if backupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "备份 ID 不能为空"})
		return
	}
	if !validateBackupID(backupID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份 ID"})
		return
	}

	// 安全检查：确保路径在备份目录内
	backupPath := filepath.Join(backupDir, backupID)
	absPath, err := filepath.Abs(backupPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份路径"})
		return
	}
	absBackupDir, _ := filepath.Abs(backupDir)
	if absPath != absBackupDir && !strings.HasPrefix(absPath, absBackupDir+string(os.PathSeparator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份路径"})
		return
	}

	// 验证备份是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份不存在"})
		return
	}

	// 删除备份目录
	if err := os.RemoveAll(backupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除备份失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "备份已删除"})
}

// CreateBackup 创建备份（供内部调用）
func CreateBackup(version string, isAuto bool) (string, error) {
	// 确保备份目录存在
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 生成备份 ID
	timestamp := time.Now().Format("20060102-150405")
	backupID := fmt.Sprintf("backup-%s-%s", version, timestamp)
	backupPath := filepath.Join(backupDir, backupID)

	// 创建备份目录
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return "", fmt.Errorf("创建备份子目录失败: %w", err)
	}

	files := []string{}

	// 1. 备份数据库 (排除发送日志等大表，减小备份体积)
	if _, err := os.Stat("goemail.db"); err == nil {
		// 生成清理过的数据库快照 (WAL checkpoint + 删除日志大表)
		if err := createCleanDBBackup(filepath.Join(backupPath, "goemail.db")); err != nil {
			return "", fmt.Errorf("备份数据库失败: %w", err)
		}
		files = append(files, "goemail.db")
	}

	// 2. 备份配置文件
	if _, err := os.Stat("config.json"); err == nil {
		if err := copyFile("config.json", filepath.Join(backupPath, "config.json")); err != nil {
			return "", fmt.Errorf("备份配置文件失败: %w", err)
		}
		files = append(files, "config.json")
	}

	// 3. 备份当前程序文件
	currentExe, err := os.Executable()
	if err == nil {
		currentExe, _ = filepath.EvalSymlinks(currentExe)
		exeBackup := filepath.Join(backupPath, "goemail.backup")
		if err := copyFile(currentExe, exeBackup); err == nil {
			files = append(files, filepath.Base(currentExe))
		}
	}

	// 4. 创建清单文件
	manifest := BackupManifest{
		ID:        backupID,
		Version:   version,
		CreatedAt: time.Now().Format(time.RFC3339),
		IsAuto:    isAuto,
		Files:     files,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("创建清单失败: %w", err)
	}

	if err := os.WriteFile(filepath.Join(backupPath, "manifest.json"), manifestData, 0644); err != nil {
		return "", fmt.Errorf("写入清单失败: %w", err)
	}

	// 5. 清理旧备份（保留最近 10 个）
	cleanOldBackups(10)

	return backupID, nil
}

// listBackups 列出所有备份
func listBackups() ([]BackupInfo, error) {
	backups := []BackupInfo{}

	// 检查备份目录是否存在
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return backups, nil
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "backup-") {
			continue
		}

		backupPath := filepath.Join(backupDir, entry.Name())
		manifestPath := filepath.Join(backupPath, "manifest.json")

		// 读取清单
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var manifest BackupManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			continue
		}

		// 计算备份大小
		size := int64(0)
		filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				size += info.Size()
			}
			return nil
		})

		createdAt, _ := time.Parse(time.RFC3339, manifest.CreatedAt)

		backups = append(backups, BackupInfo{
			ID:        manifest.ID,
			Version:   manifest.Version,
			CreatedAt: createdAt,
			Size:      size,
			IsAuto:    manifest.IsAuto,
			Files:     manifest.Files,
		})
	}

	// 按时间倒序排列
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// cleanOldBackups 清理旧备份
func cleanOldBackups(keepCount int) {
	backups, err := listBackups()
	if err != nil || len(backups) <= keepCount {
		return
	}

	// 删除超出数量的旧备份
	for i := keepCount; i < len(backups); i++ {
		backupPath := filepath.Join(backupDir, backups[i].ID)
		os.RemoveAll(backupPath)
	}
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// 复制文件权限
	sourceInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, sourceInfo.Mode())
	}

	return nil
}

// createCleanDBBackup 生成清理过的数据库备份快照
// 先 WAL checkpoint 落盘，再用 VACUUM INTO 生成一致性快照，
// 并在快照副本上删除大表数据 (发送日志/转发日志/已完成队列)，压缩后复制到目标路径。
// 目的：备份不含邮件正文等日志数据，显著减小备份体积。
func createCleanDBBackup(destPath string) error {
	// 1. WAL checkpoint，确保数据落入主库文件
	sqlDB, err := database.DB.DB()
	if err != nil {
		return err
	}
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("WAL checkpoint 失败: %w", err)
	}

	// 2. VACUUM INTO 生成一致性快照 (SQLite 官方推荐的在线备份方式)
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("goemail-backup-%d.db", time.Now().UnixNano()))
	defer os.Remove(tmpPath)

	// SQL 中路径使用正斜杠并转义单引号
	sqlPath := strings.ReplaceAll(tmpPath, "\\", "/")
	sqlPath = strings.ReplaceAll(sqlPath, "'", "''")
	if _, err := sqlDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", sqlPath)); err != nil {
		return fmt.Errorf("生成数据库快照失败: %w", err)
	}

	// 3. 打开快照副本，删除大表数据
	tmpDB, err := gorm.Open(sqlite.Open(tmpPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("打开数据库快照失败: %w", err)
	}

	tmpDB.Exec("DELETE FROM email_logs")                                        // 发送日志 (含邮件正文，占用最大)
	tmpDB.Exec("DELETE FROM forward_logs")                                      // 转发日志
	tmpDB.Exec("DELETE FROM email_queues WHERE status IN ('completed','dead')") // 已完成/最终失败的队列记录
	tmpDB.Exec("VACUUM")

	if sqlTmpDB, err := tmpDB.DB(); err == nil {
		sqlTmpDB.Close()
	}

	// 4. 复制快照到目标路径
	return copyFile(tmpPath, destPath)
}
