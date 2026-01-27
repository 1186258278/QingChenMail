package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"goemail/internal/api"
	"goemail/internal/config"
	"goemail/internal/database"
	"goemail/internal/mailer"
	"goemail/internal/receiver"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// 命令行参数
	resetPwd := flag.Bool("reset", false, "Reset admin password to 123456")
	flag.Parse()

	// 1. 加载配置
	config.LoadConfig()

	// 2. 初始化数据库
	database.InitDB()

	// 处理重置密码指令
	if *resetPwd {
		// 使用 Bcrypt 哈希存储密码
		// 为了简化运维，重置操作仍然将密码设为 123456，但存储为 Hash
		// 建议管理员在重置后立即登录并修改密码
		hashedPassword, err := database.HashPassword("123456")
		if err != nil {
			log.Fatal("Failed to hash password:", err)
		}

		var user database.User
		if err := database.DB.Where("username = ?", "admin").First(&user).Error; err == nil {
			user.Password = hashedPassword
			database.DB.Save(&user)
			fmt.Println("[SUCCESS] Admin password has been reset to: 123456")
		} else {
			// 如果用户不存在，创建它
			user = database.User{Username: "admin", Password: hashedPassword}
			database.DB.Create(&user)
			fmt.Println("[SUCCESS] Admin user created with password: 123456")
		}
		os.Exit(0)
	}

	// 启动邮件发送队列 Worker
	mailer.StartQueueWorker()

	// 启动 SMTP 接收服务 (邮件转发)
	receiver.StartReceiver()

	// 启动营销任务调度器 (定时发送)
	api.StartCampaignScheduler()

	// 3. 设置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 请求体大小限制 (32MB)
	r.MaxMultipartMemory = 32 << 20

	// 4. API 路由
	apiGroup := r.Group("/api/v1")
	{
		// 公开接口 (添加速率限制)
		apiGroup.POST("/login", api.RateLimitMiddleware(api.GetLoginLimiter()), api.LoginHandler)
		apiGroup.GET("/captcha", api.RateLimitMiddleware(api.GetCaptchaLimiter()), api.CaptchaHandler)
		apiGroup.GET("/wallpaper", api.WallpaperHandler)

		// 追踪接口 (公开)
		apiGroup.GET("/track/open/:id", api.TrackOpenHandler)
		apiGroup.GET("/track/click/:id", api.TrackClickHandler)
		apiGroup.GET("/track/unsubscribe/:id", api.UnsubscribeHandler)

		// 需要认证的接口 (支持 JWT 或 API Key)
		authorized := apiGroup.Group("/")
		authorized.Use(api.AuthMiddleware())
		{
			// 发送接口 (现在受保护)
			authorized.POST("/send", api.SendHandler)

			authorized.GET("/stats", api.StatsHandler)
			authorized.GET("/logs", api.LogsHandler)
			authorized.POST("/config/dkim", api.GenerateDKIMHandler)
			authorized.GET("/config", api.GetConfigHandler)
			authorized.GET("/config/version", api.GetVersionHandler) // 新增
			authorized.GET("/config/check-update", api.CheckUpdateHandler) // 新增：版本检查代理
			authorized.POST("/config", api.UpdateConfigHandler)
			authorized.POST("/config/test-port", api.TestPortHandler)
			authorized.POST("/config/kill-process", api.KillProcessHandler) // 新增
			authorized.POST("/password", api.ChangePasswordHandler)
			authorized.GET("/backup", api.BackupHandler)
			
			// SMTP 管理
			authorized.POST("/smtp", api.CreateSMTPHandler)
			authorized.GET("/smtp", api.ListSMTPHandler)
			authorized.PUT("/smtp/:id", api.UpdateSMTPHandler)
			authorized.DELETE("/smtp/:id", api.DeleteSMTPHandler)

			// 域名管理
			authorized.POST("/domains", api.CreateDomainHandler)
			authorized.GET("/domains", api.ListDomainHandler)
			authorized.PUT("/domains/:id", api.UpdateDomainHandler) // 新增 Update
			authorized.DELETE("/domains/:id", api.DeleteDomainHandler)
			authorized.POST("/domains/:id/verify", api.VerifyDomainHandler)

			// 模板管理
			authorized.POST("/templates", api.CreateTemplateHandler)
			authorized.GET("/templates", api.ListTemplateHandler)
			authorized.PUT("/templates/:id", api.UpdateTemplateHandler)
			authorized.DELETE("/templates/:id", api.DeleteTemplateHandler)

			// 密钥管理
			authorized.GET("/keys", api.ListAPIKeysHandler)
			authorized.POST("/keys", api.CreateAPIKeyHandler)
			authorized.DELETE("/keys/:id", api.DeleteAPIKeyHandler)

			// 文件管理
			authorized.GET("/files", api.ListFilesHandler)
			authorized.GET("/files/:id/download", api.DownloadFileHandler)
			authorized.DELETE("/files/:id", api.DeleteFileHandler)
			authorized.POST("/files/batch_delete", api.BatchDeleteFilesHandler)

			// 转发规则管理
			authorized.GET("/forward-rules", api.ListForwardRulesHandler)      // ?domain_id=xxx
			authorized.POST("/forward-rules", api.CreateForwardRuleHandler)    // body: {domain_id, ...}
			authorized.PUT("/forward-rules/:id", api.UpdateForwardRuleHandler)
			authorized.DELETE("/forward-rules/:id", api.DeleteForwardRuleHandler)
			authorized.POST("/forward-rules/:id/toggle", api.ToggleForwardRuleHandler)
			
			// 转发日志
			authorized.GET("/forward-logs", api.ListForwardLogsHandler)
			authorized.GET("/forward-stats", api.GetForwardStatsHandler)

			// 联系人管理
			authorized.GET("/contacts/groups", api.ListContactGroupsHandler)
			authorized.POST("/contacts/groups", api.CreateContactGroupHandler)
			authorized.PUT("/contacts/groups/:id", api.UpdateContactGroupHandler)
			authorized.DELETE("/contacts/groups/:id", api.DeleteContactGroupHandler)

			authorized.GET("/contacts", api.ListContactsHandler)
			authorized.POST("/contacts", api.CreateContactHandler)
			authorized.PUT("/contacts/:id", api.UpdateContactHandler)
			authorized.DELETE("/contacts/:id", api.DeleteContactHandler)
			authorized.POST("/contacts/import", api.ImportContactsHandler)
			authorized.GET("/contacts/export", api.ExportContactsHandler)
			authorized.POST("/contacts/batch_delete", api.BatchDeleteContactsHandler)
			authorized.GET("/contacts/unsubscribed", api.ListUnsubscribedHandler)
			authorized.POST("/contacts/:id/resubscribe", api.ResubscribeHandler)

			// 营销活动管理
			authorized.GET("/campaigns", api.ListCampaignsHandler)
			authorized.POST("/campaigns", api.CreateCampaignHandler)
			authorized.PUT("/campaigns/:id", api.UpdateCampaignHandler)
			authorized.DELETE("/campaigns/:id", api.DeleteCampaignHandler)
			authorized.POST("/campaigns/:id/start", api.StartCampaignHandler)
			authorized.POST("/campaigns/:id/pause", api.PauseCampaignHandler)
			authorized.POST("/campaigns/:id/resume", api.ResumeCampaignHandler)
			authorized.GET("/campaigns/:id/progress", api.GetCampaignProgressHandler)
			authorized.POST("/campaigns/:id/test", api.TestCampaignHandler)

			// 收件箱
			authorized.GET("/inbox", api.ListInboxHandler)
			authorized.GET("/inbox/stats", api.GetInboxStatsHandler)
			authorized.GET("/inbox/:id", api.GetInboxItemHandler)
			authorized.GET("/inbox/:id/attachments", api.GetInboxAttachmentsHandler)
			authorized.DELETE("/inbox/:id", api.DeleteInboxItemHandler)
			authorized.POST("/inbox/batch/read", api.BatchMarkReadHandler)
			authorized.POST("/inbox/batch/delete", api.BatchDeleteHandler)

			// 收件配置
			authorized.GET("/receiver/config", api.GetReceiverConfigHandler)
			authorized.PUT("/receiver/config", api.UpdateReceiverConfigHandler)
		}
	}

	// 5. 静态文件服务
	// 嵌入式静态文件 (UI)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	r.StaticFS("/dashboard", http.FS(staticFS))
	
	// 本地静态文件 (壁纸缓存)
	// 挂载到 /wallpapers 路径，避免与 /dashboard 通配符冲突
	r.Static("/wallpapers", "./static/wallpapers")
	// 兼容根路径资源请求 (fix 404)
	r.GET("/css/*filepath", func(c *gin.Context) {
		c.FileFromFS("static/css/"+c.Param("filepath"), http.FS(staticFiles))
	})
	r.GET("/js/*filepath", func(c *gin.Context) {
		c.FileFromFS("static/js/"+c.Param("filepath"), http.FS(staticFiles))
	})

	// 单独提供 login.html，方便访问
	r.GET("/login.html", func(c *gin.Context) {
		c.FileFromFS("static/login.html", http.FS(staticFiles))
	})

	// 根路径重定向到 dashboard
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/dashboard/")
	})

	// 6. 启动服务
	port := config.AppConfig.Port
	if port == "" {
		port = "9901"
	}
	host := config.AppConfig.Host
	if host == "" {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%s", host, port)
	
	fmt.Printf("QingChen Mail server starting on %s...\n", addr)
	
	if config.AppConfig.EnableSSL && config.AppConfig.CertFile != "" && config.AppConfig.KeyFile != "" {
		fmt.Printf("SSL Enabled. Dashboard: https://%s:%s/dashboard/\n", host, port)
		if err := r.RunTLS(addr, config.AppConfig.CertFile, config.AppConfig.KeyFile); err != nil {
			log.Fatal("SSL Start Failed: ", err)
		}
	} else {
		fmt.Printf("Dashboard: http://%s:%s/dashboard/\n", host, port)
		if err := r.Run(addr); err != nil {
			log.Fatal(err)
		}
	}
}
