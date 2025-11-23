package entity

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sharath018/temple-management-backend/internal/auth"
	"github.com/sharath018/temple-management-backend/middleware"
)

type Handler struct {
	Service   *Service
	UploadDir string // filesystem base, e.g. "./uploads"
	BaseURL   string // URL base, e.g. "/api/v1/uploads"
	MaxSize   int64  // 10MB default
}

func NewHandler(s *Service, uploadDir, baseURL string) *Handler {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("Failed to create upload directory: %v", err)
	}
	// Sensible defaults: BaseURL should point to the secured binary route (/api/v1/uploads)
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "/files"
	}
	return &Handler{
		Service:   s,
		UploadDir: uploadDir,
		BaseURL:   baseURL,
		MaxSize:   10 * 1024 * 1024,
	}
}

// In handler.go - Update the CreateEntity function

func (h *Handler) CreateEntity(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.Contains(contentType, "multipart/form-data")

	var input Entity
	var tempFiles []TempFileInfo

	if isMultipart {
		if err := h.handleMultipartFormData(c, &input, &tempFiles); err != nil {
			log.Printf("Multipart Form Error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data", "details": err.Error()})
			return
		}
	} else {
		if err := c.ShouldBindJSON(&input); err != nil {
			log.Printf("JSON Bind Error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
			return
		}
	}

	// Required fields
	if input.TempleType == "" || input.State == "" || input.EstablishedYear == nil {
		h.cleanupTempFiles(tempFiles)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Temple Type, State, and Established Year are required"})
		return
	}
	if strings.TrimSpace(input.StreetAddress) == "" {
		h.cleanupTempFiles(tempFiles)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Street address is required"})
		return
	}

	// Auth and access
	userVal, exists := c.Get("user")
	if !exists {
		h.cleanupTempFiles(tempFiles)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObj := userVal.(auth.User)
	userID := userObj.ID
	userRole := userObj.Role.RoleName
	userRoleID := userObj.Role.ID // 🆕 GET ROLE ID

	var accessContext middleware.AccessContext
	if v, ok := c.Get("access_context"); ok {
		accessContext, _ = v.(middleware.AccessContext)
	}

	// Decide CreatedBy
	switch userRole {
	case "superadmin":
		if accessContext.AssignedEntityID != nil {
			input.CreatedBy = *accessContext.AssignedEntityID
		} else {
			tenantID, err := h.Service.Repo.GetTenantIDForUser(userID)
			if err != nil || tenantID == 0 {
				h.cleanupTempFiles(tempFiles)
				c.JSON(http.StatusForbidden, gin.H{"error": "User is not assigned to any tenant"})
				return
			}
			input.CreatedBy = tenantID
		}
	case "templeadmin":
		input.CreatedBy = userID
	case "standarduser", "monitoringuser":
		if accessContext.AssignedEntityID != nil {
			input.CreatedBy = *accessContext.AssignedEntityID
		} else {
			tenantID, err := h.Service.Repo.GetTenantIDForUser(userID)
			if err != nil || tenantID == 0 {
				h.cleanupTempFiles(tempFiles)
				c.JSON(http.StatusForbidden, gin.H{"error": "User is not assigned to any tenant"})
				return
			}
			input.CreatedBy = tenantID
		}
	default:
		h.cleanupTempFiles(tempFiles)
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user role for temple creation"})
		return
	}

	// 🆕 AUTO-APPROVE FOR SUPERADMIN (role_id = 1)
	if input.Status == "" {
		if userRoleID == 1 { // Superadmin role_id
			input.Status = "approved"
			log.Printf("Temple auto-approved: created by superadmin (role_id: %d)", userRoleID)
		} else {
			input.Status = "pending"
			log.Printf("Temple pending approval: created by role_id: %d", userRoleID)
		}
	}

	ip := middleware.GetIPFromContext(c)

	// Create entity with role_id passed to service
	if err := h.Service.CreateEntity(&input, userID, userRoleID, ip); err != nil {
		log.Printf("Service Error: %v", err)
		h.cleanupTempFiles(tempFiles)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entity", "details": err.Error()})
		return
	}
	log.Printf("Entity created successfully with ID: %d, Status: %s", input.ID, input.Status)

	// Move files and update URLs
	finalFileInfos := make(map[string]FileInfo)
	if len(tempFiles) > 0 {
		if err := h.moveFilesToFinalLocation(&input, tempFiles, &finalFileInfos); err != nil {
			log.Printf("Error moving files for entity %d: %v", input.ID, err)
			c.JSON(http.StatusCreated, gin.H{
				"message":    "Temple created but some files could not be processed",
				"temple_id":  input.ID,
				"status":     input.Status,
				"file_error": err.Error(),
			})
			return
		}
		if err := h.updateEntityWithFileInfo(&input); err != nil {
			log.Printf("Error updating entity %d with file info: %v", input.ID, err)
		}
		log.Printf("Files processed successfully for entity %d", input.ID)
	}

	// 🆕 DIFFERENT RESPONSE MESSAGES BASED ON STATUS
	responseMessage := "Temple registration request submitted successfully"
	if input.Status == "approved" {
		responseMessage = "Temple created and approved successfully"
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":        responseMessage,
		"temple_id":      input.ID,
		"status":         input.Status,
		"auto_approved":  input.Status == "approved",
		"uploaded_files": finalFileInfos,
	})
}

type TempFileInfo struct {
	TempPath     string
	FileType     string // registration_cert, trust_deed, property_docs, additional_docs
	FileName     string
	OriginalName string
	FileSize     int64
	ContentType  string
	UploadedAt   time.Time
}

func (h *Handler) handleMultipartFormData(c *gin.Context, input *Entity, tempFiles *[]TempFileInfo) error {
	form, err := c.MultipartForm()
	if err != nil {
		return fmt.Errorf("failed to parse multipart form: %v", err)
	}

	// Text fields
	input.Name = h.getFormValue(form, "name")
	if v := h.getFormValue(form, "main_deity"); v != "" {
		input.MainDeity = &v
	}
	input.TempleType = h.getFormValue(form, "temple_type")
	if yearStr := h.getFormValue(form, "established_year"); yearStr != "" {
		if year, err := strconv.ParseUint(yearStr, 10, 32); err == nil {
			yy := uint(year)
			input.EstablishedYear = &yy
		}
	}
	input.Phone = h.getFormValue(form, "phone")
	input.Email = h.getFormValue(form, "email")
	input.Description = h.getFormValue(form, "description")
	input.StreetAddress = h.getFormValue(form, "street_address")
	input.City = h.getFormValue(form, "city")
	input.District = h.getFormValue(form, "district")
	input.State = h.getFormValue(form, "state")
	input.Pincode = h.getFormValue(form, "pincode")
	input.Landmark = h.getFormValue(form, "landmark")
	input.MapLink = h.getFormValue(form, "map_link")

	if err := h.processFileUploadsToTemp(form, tempFiles); err != nil {
		return fmt.Errorf("failed to process file uploads: %v", err)
	}
	return nil
}

func (h *Handler) processFileUploadsToTemp(form *multipart.Form, tempFiles *[]TempFileInfo) error {
	tempSessionDir := filepath.Join(h.UploadDir, "temp_uploads", uuid.New().String())
	if err := os.MkdirAll(tempSessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	log.Printf("Created temp directory: %s", tempSessionDir)

	// Single-file fields
	if reg := form.File["registration_cert"]; len(reg) > 0 {
		info, err := h.uploadFileToTemp(reg[0], tempSessionDir, "registration_cert")
		if err != nil {
			return fmt.Errorf("failed to upload registration certificate: %v", err)
		}
		*tempFiles = append(*tempFiles, info)
	}
	if trust := form.File["trust_deed"]; len(trust) > 0 {
		info, err := h.uploadFileToTemp(trust[0], tempSessionDir, "trust_deed")
		if err != nil {
			return fmt.Errorf("failed to upload trust deed: %v", err)
		}
		*tempFiles = append(*tempFiles, info)
	}
	if prop := form.File["property_docs"]; len(prop) > 0 {
		info, err := h.uploadFileToTemp(prop[0], tempSessionDir, "property_docs")
		if err != nil {
			return fmt.Errorf("failed to upload property documents: %v", err)
		}
		*tempFiles = append(*tempFiles, info)
	}

	// Multiple additional docs: additional_docs_0..9
	for i := 0; i < 10; i++ {
		field := fmt.Sprintf("additional_docs_%d", i)
		if add := form.File[field]; len(add) > 0 {
			info, err := h.uploadFileToTemp(add[0], tempSessionDir, "additional_docs")
			if err != nil {
				log.Printf("Warning: Failed to upload additional document %d: %v", i, err)
				continue
			}
			*tempFiles = append(*tempFiles, info)
		}
	}
	log.Printf("Total temp files processed: %d", len(*tempFiles))
	return nil
}

func (h *Handler) uploadFileToTemp(file *multipart.FileHeader, tempDir, fileType string) (TempFileInfo, error) {
	var out TempFileInfo

	if err := h.validateFile(file); err != nil {
		return out, err
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	fileName := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	tempPath := filepath.Join(tempDir, fileName)

	src, err := file.Open()
	if err != nil {
		return out, fmt.Errorf("failed to open uploaded file: %v", err)
	}
	defer src.Close()

	dst, err := os.Create(tempPath)
	if err != nil {
		return out, fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return out, fmt.Errorf("failed to copy file: %v", err)
	}

	out = TempFileInfo{
		TempPath:     tempPath,
		FileType:     fileType,
		FileName:     fileName,
		OriginalName: file.Filename,
		FileSize:     file.Size,
		ContentType:  sniffOrByExt(ext),
		UploadedAt:   time.Now(),
	}
	return out, nil
}

func (h *Handler) moveFilesToFinalLocation(entity *Entity, tempFiles []TempFileInfo, finalFileInfos *map[string]FileInfo) error {
	entityDir := filepath.Join(h.UploadDir, strconv.FormatUint(uint64(entity.ID), 10))
	if err := os.MkdirAll(entityDir, 0755); err != nil {
		return fmt.Errorf("failed to create entity directory: %v", err)
	}
	log.Printf("Created entity directory: %s for entity %d", entityDir, entity.ID)

	*finalFileInfos = make(map[string]FileInfo)
	var additionalFiles []FileInfo

	for _, tf := range tempFiles {
		finalFileName := tf.FileName
		finalPath := filepath.Join(entityDir, finalFileName)

		// Prefer rename; fall back to copy+remove
		if err := os.Rename(tf.TempPath, finalPath); err != nil {
			if err := copyFile(tf.TempPath, finalPath); err != nil {
				log.Printf("Failed to move/copy file %s to %s: %v", tf.TempPath, finalPath, err)
				return fmt.Errorf("failed to persist file %s: %v", tf.FileName, err)
			}
			_ = os.Remove(tf.TempPath)
		}

		rel := filepath.ToSlash(filepath.Join(strconv.FormatUint(uint64(entity.ID), 10), finalFileName))
		fileURL := h.buildFileURL(rel)

		fi := FileInfo{
			FileName:     finalFileName,
			FileURL:      fileURL,
			FileSize:     tf.FileSize,
			FileType:     tf.ContentType,
			UploadedAt:   tf.UploadedAt,
			OriginalName: tf.OriginalName,
		}

		switch tf.FileType {
		case "registration_cert":
			(*finalFileInfos)["registration_cert"] = fi
			entity.RegistrationCertURL = fileURL
			if b, err := json.Marshal(fi); err == nil {
				entity.RegistrationCertInfo = string(b)
			}
		case "trust_deed":
			(*finalFileInfos)["trust_deed"] = fi
			entity.TrustDeedURL = fileURL
			if b, err := json.Marshal(fi); err == nil {
				entity.TrustDeedInfo = string(b)
			}
		case "property_docs":
			(*finalFileInfos)["property_docs"] = fi
			entity.PropertyDocsURL = fileURL
			if b, err := json.Marshal(fi); err == nil {
				entity.PropertyDocsInfo = string(b)
			}
		case "additional_docs":
			additionalFiles = append(additionalFiles, fi)
		}
	}

	// Persist additional as arrays
	if len(additionalFiles) > 0 {
		var urlList []string
		for _, x := range additionalFiles {
			urlList = append(urlList, x.FileURL)
		}
		if b, err := json.Marshal(urlList); err == nil {
			entity.AdditionalDocsURLs = string(b)
		}
		if b, err := json.Marshal(additionalFiles); err == nil {
			entity.AdditionalDocsInfo = string(b)
		}
		(*finalFileInfos)["additional_docs"] = FileInfo{
			FileName: fmt.Sprintf("%d_additional_files", len(additionalFiles)),
			FileURL:  "",
		}
	}

	// Clean temp
	h.cleanupTempFiles(tempFiles)

	log.Printf("Successfully processed %d files for entity %d", len(tempFiles), entity.ID)
	return nil
}

func (h *Handler) updateEntityWithFileInfo(entity *Entity) error {
	return h.Service.Repo.UpdateEntity(*entity)
}

func (h *Handler) validateFile(file *multipart.FileHeader) error {
	if file.Size > h.MaxSize {
		return fmt.Errorf("file size exceeds %dMB limit", h.MaxSize/(1024*1024))
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{
		".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".doc": true, ".docx": true,
	}
	if !allowed[ext] {
		return fmt.Errorf("file type %s not allowed", ext)
	}
	return nil
}

func (h *Handler) getFormValue(form *multipart.Form, key string) string {
	if v, ok := form.Value[key]; ok && len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}

func (h *Handler) cleanupTempFiles(tempFiles []TempFileInfo) {
	for _, tf := range tempFiles {
		_ = os.Remove(tf.TempPath)
		// try remove dir if empty
		_ = os.Remove(filepath.Dir(tf.TempPath))
	}
}

// FIXED: Build a file URL from a relative upload path like "<entityID>/<file>"
func (h *Handler) buildFileURL(rel string) string {
	// Clean the relative path
	rel = strings.TrimLeft(rel, "/")
	
	// For direct file access (recommended for downloads)
	return fmt.Sprintf("/files/%s", rel)
}

// ================= Directory Introspection =================

type EntityDirectory struct {
	EntityID   string   `json:"entity_id"`
	FilesCount int      `json:"files_count"`
	Files      []string `json:"files"`
}

// Superadmin only
func (h *Handler) GetAllEntityDirectories(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObj := userVal.(auth.User)
	if userObj.Role.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superadmins can view all entity directories"})
		return
	}

	var directories []EntityDirectory
	entries, err := os.ReadDir(h.UploadDir)
	if err != nil {
		log.Printf("Error reading upload directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read upload directory"})
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "temp_uploads" {
			entityDir := filepath.Join(h.UploadDir, entry.Name())
			files, err := os.ReadDir(entityDir)
			if err != nil {
				log.Printf("Error reading entity directory %s: %v", entry.Name(), err)
				continue
			}
			var names []string
			for _, f := range files {
				if !f.IsDir() {
					names = append(names, f.Name())
				}
			}
			if len(names) > 0 {
				directories = append(directories, EntityDirectory{
					EntityID:   entry.Name(),
					FilesCount: len(names),
					Files:      names,
				})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total_entities_with_files": len(directories),
		"directories":               directories,
	})
}

type FileDetails struct {
	FileName string `json:"file_name"`
	FileURL  string `json:"file_url"`
	Size     int64  `json:"size"`
}

// Requires temple access
func (h *Handler) GetEntityFiles(c *gin.Context) {
	entityID := c.Param("id")

	accessVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessCtx, ok := accessVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	idInt, err := strconv.Atoi(entityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}
	entityIDUint := uint(idInt)

	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObj := userVal.(auth.User)

	hasAccess := false
	switch userObj.Role.RoleName {
	case "superadmin":
		hasAccess = true
	case "templeadmin":
		hasAccess = (accessCtx.DirectEntityID != nil && *accessCtx.DirectEntityID == entityIDUint)
	case "standarduser", "monitoringuser":
		hasAccess = (accessCtx.AssignedEntityID != nil && *accessCtx.AssignedEntityID == entityIDUint)
	}
	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to files for this entity"})
		return
	}

	entityDir := filepath.Join(h.UploadDir, entityID)
	if _, err := os.Stat(entityDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "No files found for this entity"})
		return
	}

	entries, err := os.ReadDir(entityDir)
	if err != nil {
		log.Printf("Error reading entity directory %s: %v", entityID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read entity files"})
		return
	}

	var out []FileDetails
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(entityID, e.Name()))
		url := h.buildFileURL(rel)
		out = append(out, FileDetails{
			FileName: e.Name(),
			FileURL:  url,
			Size:     info.Size(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"entity_id":   entityID,
		"files_count": len(out),
		"files":       out,
	})
}

// ===== helpers =====

func sniffOrByExt(ext string) string {
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	// Default for unknown
	return "application/octet-stream"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// Rest of your existing methods remain the same...
// GetAllEntities retrieves entities based on user role and permissions
func (h *Handler) GetAllEntities(c *gin.Context) {
	// Get authenticated user
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	
	user, ok := userVal.(auth.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user object"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	var entities []Entity
	var err error

	// Role-based entity retrieval
	switch user.Role.RoleName {
	case "superadmin":
		// Super admins get all entities
		entities, err = h.Service.GetAllEntities()
		
	case "templeadmin":
		// Temple admins get entities they created
		entities, err = h.Service.GetEntitiesByCreator(user.ID)
		if err != nil || len(entities) == 0 {
			log.Printf("No entities found for templeadmin %d, returning empty list", user.ID)
			entities = []Entity{} // Return empty array instead of nil
		}
		
	case "standarduser", "monitoringuser":
		// For standard users, try multiple strategies to find entities
		if accessContext.AssignedEntityID != nil {
			tenantID := *accessContext.AssignedEntityID
			
			// Try to get entities created by the tenant
			entities, err = h.Service.GetEntitiesByCreator(tenantID)
			
			// If no entities found, create a mock entity for UI consistency
			if err != nil || len(entities) == 0 {
				log.Printf("No entities found for tenant %d, creating mock entity", tenantID)
				mockEntity := Entity{
					ID:          tenantID,
					Name:        "Temple " + strconv.FormatUint(uint64(tenantID), 10),
					Description: "Temple associated with your account",
					Status:      "active",
					CreatedBy:   tenantID,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				entities = []Entity{mockEntity}
				err = nil // Clear any error
			}
		} else {
			c.JSON(http.StatusForbidden, gin.H{"error": "No entity assigned to this user"})
			return
		}
		
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user role"})
		return
	}

	if err != nil {
		log.Printf("Error fetching entities: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch temples", "details": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, entities)
}

// GetEntityByID retrieves a specific entity by ID with permission checks
func (h *Handler) GetEntityByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Get user info
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	user, ok := userVal.(auth.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user object"})
		return
	}
	
	// Try to get the entity first
	entity, err := h.Service.GetEntityByID(id)
	if err != nil {
		// For standard users with matching tenant ID, create mock entity
		if (user.Role.RoleName == "standarduser" || user.Role.RoleName == "monitoringuser") && 
		   accessContext.AssignedEntityID != nil && 
		   *accessContext.AssignedEntityID == uint(id) {
			
			// Create mock entity using the ID
			entity = Entity{
				ID:          uint(id),
				Name:        "Temple " + strconv.Itoa(id),
				Description: "Temple associated with your account",
				Status:      "active",
				CreatedBy:   uint(id),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "Temple not found"})
			return
		}
	}
	
	// Check permissions based on user role
	hasAccess := false
	
	switch user.Role.RoleName {
	case "superadmin":
		hasAccess = true
		
	case "templeadmin":
		hasAccess = (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == uint(id)) || 
			entity.CreatedBy == user.ID
			
	case "standarduser", "monitoringuser":
		if accessContext.AssignedEntityID != nil {
			hasAccess = (*accessContext.AssignedEntityID == uint(id)) || 
				entity.CreatedBy == *accessContext.AssignedEntityID
		}
		
	default:
		hasAccess = false
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this entity"})
		return
	}

	c.JSON(http.StatusOK, entity)
}

// UpdateEntity handles entity updates with permission checks and proper field preservation
// UpdateEntity handles entity updates with permission checks and file uploads
func (h *Handler) UpdateEntity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get authenticated user
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user, ok := userVal.(auth.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user object"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Get the existing entity to check ownership and status
	existingEntity, err := h.Service.GetEntityByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Temple not found"})
		return
	}

	// Permission check
	hasAccess := false
	
	switch user.Role.RoleName {
	case "superadmin":
		hasAccess = true
		log.Printf("✅ SuperAdmin %d granted access to edit entity %d", user.ID, id)
		
	case "templeadmin":
		hasAccess = (existingEntity.CreatedBy == user.ID)
		
		if !hasAccess && accessContext.DirectEntityID != nil {
			hasAccess = (*accessContext.DirectEntityID == uint(id))
		}
		
		if hasAccess {
			log.Printf("✅ TempleAdmin %d granted access to edit entity %d", user.ID, id)
		} else {
			log.Printf("❌ TempleAdmin %d DENIED access to entity %d", user.ID, id)
		}
		
	case "standarduser", "monitoringuser":
		entityIDUint := uint(id)
		hasAccess = (accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityIDUint)
		
		if hasAccess {
			log.Printf("✅ StandardUser %d granted access to edit entity %d", user.ID, id)
		} else {
			log.Printf("❌ StandardUser %d DENIED access to entity %d", user.ID, id)
		}
		
	default:
		log.Printf("❌ Unknown role '%s' for user %d", user.Role.RoleName, user.ID)
		hasAccess = false
	}

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied to update this entity",
		})
		return
	}

	// Check write permissions
	if !accessContext.CanWrite() {
		log.Printf("❌ User %d has no write permissions", user.ID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient write permissions"})
		return
	}

	// 🆕 CHECK CONTENT TYPE - Handle both JSON and multipart/form-data
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.Contains(contentType, "multipart/form-data")

	var input Entity
	var tempFiles []TempFileInfo

	if isMultipart {
		// Handle multipart form with files
		log.Printf("📁 Processing multipart form data for entity %d", id)
		if err := h.handleMultipartFormData(c, &input, &tempFiles); err != nil {
			log.Printf("Multipart Form Error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data", "details": err.Error()})
			return
		}
	} else {
		// Handle JSON update (no files)
		if err := c.ShouldBindJSON(&input); err != nil {
			log.Printf("Update Bind Error: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input", "details": err.Error()})
			return
		}
	}

	// 🔍 DEBUG: Log received input
	log.Printf("📝 Received update data for temple %d: Name=%s, Email=%s", id, input.Name, input.Email)

	// Check if temple was rejected
	wasRejected := existingEntity.Status == "rejected"
	
	// Preserve critical fields
	input.ID = uint(id)
	input.CreatedBy = existingEntity.CreatedBy
	input.CreatedAt = existingEntity.CreatedAt
	input.UpdatedAt = time.Now()
	
	// 🆕 PRESERVE EXISTING FILE URLS IF NO NEW FILES UPLOADED
	if input.RegistrationCertURL == "" {
		input.RegistrationCertURL = existingEntity.RegistrationCertURL
		input.RegistrationCertInfo = existingEntity.RegistrationCertInfo
	}
	if input.TrustDeedURL == "" {
		input.TrustDeedURL = existingEntity.TrustDeedURL
		input.TrustDeedInfo = existingEntity.TrustDeedInfo
	}
	if input.PropertyDocsURL == "" {
		input.PropertyDocsURL = existingEntity.PropertyDocsURL
		input.PropertyDocsInfo = existingEntity.PropertyDocsInfo
	}
	if input.AdditionalDocsURLs == "" {
		input.AdditionalDocsURLs = existingEntity.AdditionalDocsURLs
		input.AdditionalDocsInfo = existingEntity.AdditionalDocsInfo
	}
	
	// Preserve creator role ID
	if input.CreatorRoleID == nil {
		input.CreatorRoleID = existingEntity.CreatorRoleID
	}
	
	// Handle status based on role and rejection state
	if wasRejected && user.Role.RoleName != "superadmin" {
		input.Status = "pending"
		log.Printf("🔄 Temple %d was rejected, resetting to pending for re-approval", id)
	} else if user.Role.RoleName != "superadmin" {
		input.Status = existingEntity.Status
	}
	// Superadmin can set any status

	// Preserve IsActive
	if !input.IsActive && existingEntity.IsActive {
		input.IsActive = existingEntity.IsActive
	}

	// 🆕 PROCESS NEW FILES IF UPLOADED
	finalFileInfos := make(map[string]FileInfo)
	if len(tempFiles) > 0 {
		log.Printf("📁 Processing %d new file uploads for entity %d", len(tempFiles), id)
		
		if err := h.moveFilesToFinalLocation(&input, tempFiles, &finalFileInfos); err != nil {
			log.Printf("Error moving files for entity %d: %v", id, err)
			h.cleanupTempFiles(tempFiles)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to process uploaded files",
				"details": err.Error(),
			})
			return
		}
		
		log.Printf("✅ Successfully processed %d files for entity %d", len(tempFiles), id)
	}

	// Get IP address for audit logging
	ip := middleware.GetIPFromContext(c)

	// 🔍 DEBUG: Log before save
	log.Printf("💾 Saving entity %d: Status=%s, RegCert=%s", 
		id, input.Status, input.RegistrationCertURL)

	// Perform the update
	if err := h.Service.UpdateEntity(input, user.ID, user.Role.ID, ip, wasRejected); err != nil {
		log.Printf("❌ Update Error for entity %d: %v", id, err)
		h.cleanupTempFiles(tempFiles)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update temple", 
			"details": err.Error(),
		})
		return
	}

	log.Printf("✅ Entity %d updated successfully by user %d", id, user.ID)
	
	response := gin.H{
		"message":   "Temple updated successfully",
		"temple_id": id,
		"updated_by": user.ID,
	}
	
	// Add file info if files were uploaded
	if len(finalFileInfos) > 0 {
		response["uploaded_files"] = finalFileInfos
	}
	
	// Add status change info if applicable
	if wasRejected && input.Status == "pending" {
		response["status_changed"] = true
		response["new_status"] = "pending"
		response["message"] = "Temple updated and submitted for re-approval"
	}
	
	c.JSON(http.StatusOK, response)
}
// DeleteEntity handles entity deletion (superadmin only)
func (h *Handler) DeleteEntity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get authenticated user
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObj := user.(auth.User)
	userID := userObj.ID

	// Check if user is superadmin (only superadmins should delete entities)
	if userObj.Role.RoleName != "superadmin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only superadmins can delete temples"})
		return
	}

	// Get IP address for audit logging
	ip := middleware.GetIPFromContext(c)

	if err := h.Service.DeleteEntity(id, userID, ip); err != nil {
		log.Printf("Delete Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete temple", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Temple deleted successfully"})
}
// ToggleEntityStatus handles toggling entity active/inactive status
func (h *Handler) ToggleEntityStatus(c *gin.Context) {
    id, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
        return
    }

    // Get authenticated user
    userVal, exists := c.Get("user")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    user, ok := userVal.(auth.User)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user object"})
        return
    }

    // Get the entity first to check ownership
    existingEntity, err := h.Service.GetEntityByID(id)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Temple not found"})
        return
    }

    // Check permissions based on user role
    hasAccess := false
    
    switch user.Role.RoleName {
    case "superadmin":
        // SuperAdmin can toggle any temple
        hasAccess = true
        
    case "templeadmin":
        // Temple admin can only toggle temples they created
        hasAccess = (existingEntity.CreatedBy == user.ID)
        
    case "standarduser", "monitoringuser":
        // Standard/monitoring users cannot toggle status
        c.JSON(http.StatusForbidden, gin.H{
            "error": "Insufficient permissions to toggle temple status",
            "message": "Only temple creators and administrators can change temple status",
        })
        return
        
    default:
        c.JSON(http.StatusForbidden, gin.H{"error": "Invalid user role"})
        return
    }

    if !hasAccess {
        c.JSON(http.StatusForbidden, gin.H{
            "error": "Access denied to toggle status for this temple",
            "message": "You can only toggle status for temples you created",
        })
        return
    }

    // Parse request body
    var req struct {
        IsActive bool `json:"isactive"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
        return
    }

    // Get IP address for audit logging
    ip := middleware.GetIPFromContext(c)

    // Perform the status toggle
    if err := h.Service.ToggleEntityStatus(id, req.IsActive, user.ID, ip); err != nil {
        log.Printf("Toggle Status Error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Failed to toggle temple status",
            "details": err.Error(),
        })
        return
    }

    statusText := "inactive"
    if req.IsActive {
        statusText = "active"
    }

    c.JSON(http.StatusOK, gin.H{
        "message":   "Temple status updated successfully",
        "temple_id": id,
        "isactive":  req.IsActive,
        "status":    statusText,
    })
}
/*func (h *Handler) GetVolunteersByEntity(c *gin.Context) {
	entityIDParam := c.Param("id")
	entityIDUint, err := strconv.ParseUint(entityIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	entityID := uint(entityIDUint)

	volunteers, err := h.Service.GetVolunteersByEntityID(entityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch volunteers",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, volunteers)
}
*/
// GetDevoteesByEntity retrieves devotees for a specific entity
func (h *Handler) GetDevoteesByEntity(c *gin.Context) {
	entityIDParam := c.Param("id")
	entityIDUint, err := strconv.ParseUint(entityIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check permissions
	entityID := uint(entityIDUint)
	
	// Debug logging
	log.Printf("=== Access Check Debug ===")
	log.Printf("Requested Entity ID: %d", entityID)
	log.Printf("DirectEntityID: %v", accessContext.DirectEntityID)
	if accessContext.DirectEntityID != nil {
		log.Printf("DirectEntityID value: %d", *accessContext.DirectEntityID)
	}
	log.Printf("AssignedEntityID: %v", accessContext.AssignedEntityID)
	if accessContext.AssignedEntityID != nil {
		log.Printf("AssignedEntityID value: %d", *accessContext.AssignedEntityID)
	}
	
	hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityID) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityID)

	log.Printf("hasAccess result: %v", hasAccess)
	log.Printf("=========================")

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied to devotees for this entity",
		})
		return
	}

	// Fetch devotees for the given entity
	devotees, err := h.Service.GetDevotees(entityID)
	if err != nil {
		log.Printf("Error fetching devotees for entity %d: %v", entityID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devotees", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, devotees)
}

// GetDevoteeStats retrieves devotee statistics for an entity
func (h *Handler) GetDevoteeStats(c *gin.Context) {
	entityIDStr := c.Param("id")
	entityIDUint, err := strconv.ParseUint(entityIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
		return
	}

	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Check permissions
	entityID := uint(entityIDUint)
	hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityID) ||
		(accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityID)

	if !hasAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to devotee stats for this entity"})
		return
	}

	stats, err := h.Service.GetDevoteeStats(entityID)
	if err != nil {
		log.Printf("Error fetching devotee stats for entity %d: %v", entityID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devotee stats", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *Handler) UpdateDevoteeMembershipStatus(c *gin.Context) {

    // Debug route params
    log.Println("PARAMS DEBUG:", c.Params)

    // Correct route param names
    entityIDUint, err := strconv.ParseUint(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid entity ID"})
        return
    }

    // IMPORTANT: Use the correct param name from router → "userID"
    userIDUint, err := strconv.ParseUint(c.Param("userID"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }

    // Get access context
    accessContextVal, exists := c.Get("access_context")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
        return
    }
    accessContext, ok := accessContextVal.(middleware.AccessContext)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
        return
    }

    // Permission check - write access required
    if !accessContext.CanWrite() {
        c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient write permissions"})
        return
    }

    entityID := uint(entityIDUint)
    userID := uint(userIDUint)

    // Entity access check
    hasAccess := (accessContext.DirectEntityID != nil && *accessContext.DirectEntityID == entityID) ||
        (accessContext.AssignedEntityID != nil && *accessContext.AssignedEntityID == entityID)

    if !hasAccess {
        c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to manage devotees for this entity"})
        return
    }

    // Parse status change request
    var req struct {
        Status string `json:"status" binding:"required,oneof=active inactive"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
        return
    }

    // Call service to update status
    if err := h.Service.MembershipService.UpdateMembershipStatus(userID, entityID, req.Status); err != nil {
        log.Printf("Error updating membership status: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status", "details": err.Error()})
        return
    }

    // Success response
    c.JSON(http.StatusOK, gin.H{
        "message": "Membership status updated successfully",
    })
}


// GetDashboardSummary retrieves dashboard summary for the accessible entity
func (h *Handler) GetDashboardSummary(c *gin.Context) {
	// Get access context
	accessContextVal, exists := c.Get("access_context")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing access context"})
		return
	}
	accessContext, ok := accessContextVal.(middleware.AccessContext)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access context"})
		return
	}

	// Get the accessible entity ID
	entityID := accessContext.GetAccessibleEntityID()
	if entityID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No accessible entity found"})
		return
	}

	// Call service to get dashboard summary
	summary, err := h.Service.GetDashboardSummary(*entityID)
	if err != nil {
		log.Printf("Dashboard Summary Error for entity %d: %v", *entityID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch dashboard summary", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}