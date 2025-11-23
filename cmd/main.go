package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/sharath018/temple-management-backend/config"
	"github.com/sharath018/temple-management-backend/database"
	"github.com/sharath018/temple-management-backend/internal/auditlog"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/internal/entity"
	"github.com/sharath018/temple-management-backend/internal/event"
	"github.com/sharath018/temple-management-backend/internal/eventrsvp"
	"github.com/sharath018/temple-management-backend/internal/notification"
	"github.com/sharath018/temple-management-backend/routes"
	"github.com/sharath018/temple-management-backend/utils"
)

func main() {
	cfg := config.Load()
	db := database.Connect(cfg)

	// Init Redis
	if err := utils.InitRedis(); err != nil {
		log.Fatalf("❌ Redis init failed: %v", err)
	}

	// Init Kafka
	utils.InitializeKafka()

	// 🔥 Init Firebase - SINGLE INITIALIZATION POINT
	log.Println("🔄 Initializing Firebase...")
	if err := utils.InitFirebase(); err != nil {
		log.Printf("⚠️ Firebase initialization failed: %v", err)
		log.Println("ℹ️ Continuing without Firebase (push notifications will be disabled)")
	} else if utils.IsFCMEnabled() {
		log.Println("✅ Firebase and FCM initialized successfully")
	} else {
		log.Println("⚠️ Firebase initialized but FCM client unavailable")
	}

	// Init repositories & services
	authRepo := auth.NewRepository(db)
	auditRepo := auditlog.NewRepository(db)
	auditSvc := auditlog.NewService(auditRepo)

	notificationRepo := notification.NewRepository(db)
	notificationService := notification.NewService(notificationRepo, authRepo, cfg, auditSvc)
	notification.StartKafkaConsumer(notificationService)

	// Seed roles & super admin
	if err := auth.SeedUserRoles(db); err != nil {
		panic(fmt.Sprintf("❌ Failed to seed roles: %v", err))
	}
	if err := auth.SeedSuperAdminUser(db); err != nil {
		panic(fmt.Sprintf("❌ Failed to seed Super Admin: %v", err))
	}

	// Auto-migrate models
	log.Println("🔄 Running database migrations...")
	if err := db.AutoMigrate(
		&auditlog.AuditLog{},
		&entity.Entity{},
		&event.Event{},
		&eventrsvp.RSVP{},
		&notification.InAppNotification{},
		&notification.FCMDeviceToken{}, // ✅ Add FCM device token migration
	); err != nil {
		panic(fmt.Sprintf("❌ DB AutoMigrate failed: %v", err))
	}
	log.Println("✅ Database migrations completed")

	// Add isactive column if it doesn't exist (migration for existing databases)
	log.Println("🔄 Checking for isactive column...")
	if err := migrateIsActiveColumn(db); err != nil {
		log.Printf("⚠️ Warning: IsActive migration issue: %v", err)
	} else {
		log.Println("✅ IsActive column verified/added")
	}

	// Setup Gin router
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob("templates/*")

	// Optional request logger
	router.Use(func(c *gin.Context) {
		log.Printf("REQUEST -> 👉 %s %s from origin %s", c.Request.Method, c.Request.URL.Path, c.Request.Header.Get("Origin"))
		c.Next()
	})

	// Enhanced CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:4173", "http://127.0.0.1:4173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Tenant-ID", "Content-Length", "X-Requested-With", "Cache-Control", "Pragma", "X-Entity-ID"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type", "Content-Disposition", "Cache-Control", "Pragma", "Expires"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// ✅ CRITICAL FIX: Explicit preflight handler for SuperAdmin routes
	router.OPTIONS("/api/v1/superadmin/*path", func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigins := []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:4173",
			"http://127.0.0.1:4173",
		}

		originAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				originAllowed = true
				break
			}
		}

		if !originAllowed {
			origin = "http://localhost:4173"
		}

		log.Printf("🔧 SuperAdmin OPTIONS request from origin: %s for path: %s", origin, c.Request.URL.Path)

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Tenant-ID, Content-Length, X-Requested-With, Cache-Control, Pragma, X-Entity-ID")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Disposition, Cache-Control, Pragma, Expires")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "43200")
		c.Status(204)
	})

	// Create uploads directory
	uploadDir := "/data/uploads"

	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		panic(fmt.Sprintf("❌ Failed to create upload directory: %v", err))
	}

	// ======= FILE SERVING ROUTES =======

	// Primary route: /uploads/{entityID}/{filename}
	router.GET("/uploads/:entityID/:filename", func(c *gin.Context) {
		serveEntityFile(c, uploadDir)
	})

	// Alternative route: /files/{entityID}/{filename}
	router.GET("/files/:entityID/:filename", func(c *gin.Context) {
		serveEntityFile(c, uploadDir)
	})

	// Secure API endpoint for entity files with authentication
	router.GET("/api/v1/entities/:id/files/:filename", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Disposition")

		entityID := c.Param("id")
		filename := c.Param("filename")

		if entityID == "" || filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parameters"})
			return
		}

		filePath := filepath.Join(uploadDir, entityID, filename)
		cleanPath := filepath.Clean(filePath)
		expectedPrefix := filepath.Clean(filepath.Join(uploadDir, entityID))

		if !strings.HasPrefix(cleanPath, expectedPrefix) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		fileInfo, err := os.Stat(cleanPath)
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "File not found",
				"message": "The requested file does not exist",
				"path":    filepath.Join("/api/v1/entities", entityID, "files", filename),
			})
			return
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File access error"})
			return
		}

		setContentType(c, filename)
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		c.File(cleanPath)
		log.Printf("✅ File downloaded: %s/%s", entityID, filename)
	})

	// Bulk download all files for an entity
	router.GET("/api/v1/entities/:id/files-all", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Disposition")

		entityID := c.Param("id")
		entityDir := filepath.Join(uploadDir, entityID)

		if _, err := os.Stat(entityDir); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "No files found for this entity"})
			return
		}

		zipFileName := fmt.Sprintf("Entity_%s_Documents_%s.zip", entityID, time.Now().Format("20060102_150405"))

		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFileName))
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		zipWriter := zip.NewWriter(c.Writer)
		defer zipWriter.Close()

		err := filepath.Walk(entityDir, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("⚠️ Error walking file %s: %v", filePath, err)
				return nil
			}

			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(entityDir, filePath)
			if err != nil {
				log.Printf("⚠️ Error getting relative path for %s: %v", filePath, err)
				return nil
			}

			zipFile, err := zipWriter.Create(relPath)
			if err != nil {
				log.Printf("⚠️ Error creating zip entry for %s: %v", relPath, err)
				return nil
			}

			srcFile, err := os.Open(filePath)
			if err != nil {
				log.Printf("⚠️ Error opening file %s: %v", filePath, err)
				return nil
			}
			defer srcFile.Close()

			if _, err = io.Copy(zipFile, srcFile); err != nil {
				log.Printf("⚠️ Error copying file %s to zip: %v", filePath, err)
			}

			return nil
		})

		if err != nil {
			log.Printf("⚠️ Error creating ZIP for entity %s: %v", entityID, err)
		}

		log.Printf("✅ ZIP file created for entity %s", entityID)
	})

	// Upload endpoint
	router.POST("/upload", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")

		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{"error": "File not found in request"})
			return
		}

		filename := filepath.Base(file.Filename)
		dst := filepath.Join(uploadDir, filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to save file: %v", err)})
			return
		}

		c.JSON(200, gin.H{
			"message": fmt.Sprintf("File '%s' uploaded successfully!", filename),
			"path":    dst,
			"url":     fmt.Sprintf("/uploads/%s", filename),
		})
	})

	// Debug: list all entity files
	router.GET("/debug/entity-files", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")

		type EntityFileInfo struct {
			EntityID   string   `json:"entity_id"`
			FilesCount int      `json:"files_count"`
			Files      []string `json:"files"`
			TotalSize  int64    `json:"total_size"`
		}
		var entityFiles []EntityFileInfo

		entries, err := os.ReadDir(uploadDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read upload directory"})
			return
		}

		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "temp_uploads" {
				entityDir := filepath.Join(uploadDir, entry.Name())
				files, _ := os.ReadDir(entityDir)
				var fileNames []string
				var totalSize int64
				for _, f := range files {
					if !f.IsDir() {
						fileNames = append(fileNames, f.Name())
						if info, err := f.Info(); err == nil {
							totalSize += info.Size()
						}
					}
				}
				if len(fileNames) > 0 {
					entityFiles = append(entityFiles, EntityFileInfo{
						EntityID:   entry.Name(),
						FilesCount: len(fileNames),
						Files:      fileNames,
						TotalSize:  totalSize,
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"total_entities_with_files": len(entityFiles),
			"entity_files":              entityFiles,
		})
	})

	// File info for a specific entity
	router.GET("/api/v1/entities/:id/files/info", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Credentials", "true")

		entityID := c.Param("id")
		entityDir := filepath.Join(uploadDir, entityID)

		if _, err := os.Stat(entityDir); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "No files found for this entity"})
			return
		}

		files, err := os.ReadDir(entityDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read entity files"})
			return
		}

		type FileInfo struct {
			FileName    string `json:"file_name"`
			Size        int64  `json:"size"`
			ModTime     string `json:"modified_time"`
			FileType    string `json:"file_type"`
			ViewURL     string `json:"view_url"`
			DownloadURL string `json:"download_url"`
		}

		var fileInfos []FileInfo
		var totalSize int64

		for _, file := range files {
			if !file.IsDir() {
				info, err := file.Info()
				if err != nil {
					continue
				}
				ext := strings.ToLower(filepath.Ext(file.Name()))
				fileType := strings.ToUpper(strings.TrimPrefix(ext, "."))
				if fileType == "" {
					fileType = "UNKNOWN"
				}
				fileInfos = append(fileInfos, FileInfo{
					FileName:    file.Name(),
					Size:        info.Size(),
					ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
					FileType:    fileType,
					ViewURL:     fmt.Sprintf("/uploads/%s/%s", entityID, file.Name()),
					DownloadURL: fmt.Sprintf("/api/v1/entities/%s/files/%s", entityID, file.Name()),
				})
				totalSize += info.Size()
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"entity_id":         entityID,
			"files_count":       len(fileInfos),
			"total_size":        totalSize,
			"files":             fileInfos,
			"bulk_download_url": fmt.Sprintf("/api/v1/entities/%s/files-all", entityID),
		})
	})

	// Register existing routes
	routes.Setup(router, cfg)

	// Start server
	fmt.Printf("🚀 Server starting on port %s\n", cfg.Port)
	fmt.Printf("📁 Upload directory: %s\n", uploadDir)
	fmt.Printf("🌐 File access: http://localhost:%s/uploads/{entityID}/{filename}\n", cfg.Port)
	fmt.Printf("📥 Download file: http://localhost:%s/api/v1/entities/{id}/files/{filename}\n", cfg.Port)
	fmt.Printf("📦 Bulk download: http://localhost:%s/api/v1/entities/{id}/files-all\n", cfg.Port)
	fmt.Printf("✅ CORS configured for: localhost:4173, localhost:5173\n")
	fmt.Printf("✅ PATCH method enabled for approvals\n")
	
	if utils.IsFCMEnabled() {
		fmt.Println("✅ Firebase Cloud Messaging enabled")
	} else {
		fmt.Println("ℹ️ Firebase Cloud Messaging disabled")
		if err := utils.GetInitError(); err != nil {
			fmt.Printf("   Reason: %v\n", err)
		}
	}

	if err := router.Run(":" + cfg.Port); err != nil {
		panic(fmt.Sprintf("Failed to start server: %v", err))
	}
}

// serveEntityFile handles serving files from entity directories
func serveEntityFile(c *gin.Context, uploadDir string) {
	c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Content-Disposition")

	entityID := c.Param("entityID")
	filename := c.Param("filename")

	if entityID == "" || filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parameters"})
		return
	}

	filePath := filepath.Join(uploadDir, entityID, filename)
	cleanPath := filepath.Clean(filePath)

	if !strings.HasPrefix(cleanPath, filepath.Clean(uploadDir)) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Access denied",
			"message": "Invalid file path",
		})
		return
	}

	fileInfo, err := os.Stat(cleanPath)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "File not found",
			"message": "The requested file does not exist or has been moved",
			"path":    filepath.Join("/uploads", entityID, filename),
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "File access error",
			"message": err.Error(),
		})
		return
	}

	contentType := setContentType(c, filename)
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	c.Header("Cache-Control", "public, max-age=3600")

	if strings.HasPrefix(contentType, "image/") || contentType == "application/pdf" {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	} else {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	}

	c.File(cleanPath)
	log.Printf("✅ File served: %s/%s", entityID, filename)
}

func setContentType(c *gin.Context, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := "application/octet-stream"

	switch ext {
	case ".pdf":
		contentType = "application/pdf"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".svg":
		contentType = "image/svg+xml"
	case ".doc":
		contentType = "application/msword"
	case ".docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		contentType = "application/vnd.ms-excel"
	case ".xlsx":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".txt":
		contentType = "text/plain"
	case ".csv":
		contentType = "text/csv"
	}

	c.Header("Content-Type", contentType)
	return contentType
}

func migrateIsActiveColumn(db *gorm.DB) error {
	var count int64
	err := db.Raw(`
		SELECT COUNT(*) 
		FROM information_schema.columns 
		WHERE table_name = 'entities' 
		AND column_name = 'isactive'
	`).Scan(&count).Error

	if err != nil {
		return fmt.Errorf("failed to check for isactive column: %v", err)
	}

	if count > 0 {
		log.Println("✅ IsActive column already exists")
		return nil
	}

	log.Println("🔄 Adding isactive column to entities table...")
	sql := `ALTER TABLE entities ADD COLUMN isactive BOOLEAN DEFAULT true NOT NULL;`

	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("failed to add isactive column: %v", err)
	}

	log.Println("🔄 Creating index on isactive column...")
	indexSQL := `CREATE INDEX IF NOT EXISTS idx_entities_isactive ON entities(isactive);`

	if err := db.Exec(indexSQL).Error; err != nil {
		log.Printf("⚠️ Warning: Could not create index: %v", err)
	}

	log.Println("🔄 Updating existing records with isactive = true...")
	updateSQL := `UPDATE entities SET isactive = true WHERE isactive IS NULL;`

	result := db.Exec(updateSQL)
	if result.Error != nil {
		log.Printf("⚠️ Warning: Could not update existing records: %v", result.Error)
	} else {
		log.Printf("✅ Updated %d existing records", result.RowsAffected)
	}

	log.Println("✅ IsActive column added successfully")
	return nil
}