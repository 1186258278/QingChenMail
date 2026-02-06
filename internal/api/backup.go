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

	"github.com/gin-gonic/gin"
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
				"message": "备份创建成功",
				"backup":  b,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "备份创建成功",
		"backup_id": backupID,
	})
}

// RestoreBackupHandler 恢复到指定备份
func RestoreBackupHandler(c *gin.Context) {
	backupID := c.Param("id")
	if backupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "备份 ID 不能为空"})
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
	if !strings.HasPrefix(absPath, absBackupDir) {
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
		"message":       "恢复成功",
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

	// 安全检查：确保路径在备份目录内
	backupPath := filepath.Join(backupDir, backupID)
	absPath, err := filepath.Abs(backupPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份路径"})
		return
	}
	absBackupDir, _ := filepath.Abs(backupDir)
	if !strings.HasPrefix(absPath, absBackupDir) {
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

	// 1. 备份数据库
	if _, err := os.Stat("goemail.db"); err == nil {
		if err := copyFile("goemail.db", filepath.Join(backupPath, "goemail.db")); err != nil {
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
