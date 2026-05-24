package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"

	"lab1/internal/app/repository"
	"lab1/internal/app/storage"
	"lab1/internal/app/userctx"
)

// RegisterAPI attaches REST endpoints under /api.
func (h *Handler) RegisterAPI(router *gin.Engine) {
	api := router.Group("/api")

	// Documents
	api.GET("/documents", h.apiGetDocuments)
	api.GET("/documents/:id", h.apiGetDocument)
	api.POST("/documents", h.apiCreateDocument)
	api.PUT("/documents/:id", h.apiUpdateDocument)
	api.DELETE("/documents/:id", h.apiDeleteDocument)
	api.POST("/documents/:id/image", h.apiUploadDocumentImage)

	// Add to draft (auto-create draft, по ТЗ 4.1.13)
	api.POST("/document-requests/add-to-draft", h.apiAddDocumentToDraft)
	// Add document to specific request (по ТЗ 4.1.12)
	api.POST("/document-requests/:id/documents/:documentId", h.apiAddDocumentToRequest)

	// Cart
	api.GET("/cart-icon", h.apiGetCartIcon)

	// AccessRequests
	api.GET("/document-requests", h.apiListAccessRequests)
	api.GET("/document-requests/:id", h.apiGetAccessRequest)
	api.PUT("/document-requests/:id", h.apiUpdateAccessRequest)
	api.PUT("/document-requests/:id/form", h.apiSubmitAccessRequest)
	api.PUT("/document-requests/:id/complete", h.apiCompleteAccessRequest)
	api.PUT("/document-requests/:id/reject", h.apiRejectAccessRequest)
	api.DELETE("/document-requests/:id", h.apiDeleteAccessRequestAPI)

	// AccessRequest documents (m-m)
	api.PUT("/document-requests/:id/documents/:documentId", h.apiUpdateRequestDocument)
	api.DELETE("/document-requests/:id/documents/:documentId", h.apiDeleteRequestDocument)

	// Users
	api.POST("/sign_up", h.apiRegisterUser)
	api.POST("/login", h.apiLogin)
	api.POST("/logout", h.apiLogout)
	api.GET("/profile", h.apiGetProfile)
	api.PUT("/profile", h.apiUpdateProfile)
}

// ===== Documents =====
func (h *Handler) apiGetDocuments(ctx *gin.Context) {
	status := ctx.Query("status")
	name := ctx.Query("q")

	documents, err := h.Repository.GetDocumentsFiltered(name, status)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusOK, documents)
}

func (h *Handler) apiGetDocument(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	svc, err := h.Repository.GetDocumentByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	ctx.JSON(http.StatusOK, svc)
}

type documentPayload struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ImageURL    string `json:"image_url"`
}

func (h *Handler) apiCreateDocument(ctx *gin.Context) {
	var req documentPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	req.Title = sanitizeString(req.Title)
	req.Type = sanitizeString(req.Type)
	req.Description = sanitizeString(req.Description)
	req.Status = sanitizeString(req.Status)
	svc := repository.Document{
		Title:       req.Title,
		Type:        req.Type,
		Description: req.Description,
		Status:      req.Status,
		ImageURL:    req.ImageURL,
		CreatorID:   userctx.CurrentUserID(),
	}
	if svc.Status == "" {
		svc.Status = "active"
	}
	created, err := h.Repository.CreateDocument(svc)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusCreated, created)
}

func (h *Handler) apiUpdateDocument(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var req documentPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	req.Title = sanitizeString(req.Title)
	req.Type = sanitizeString(req.Type)
	req.Description = sanitizeString(req.Description)
	req.Status = sanitizeString(req.Status)
	if err := h.Repository.UpdateDocument(uint(id), req.Title, req.Type, req.Description, req.Status); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) apiDeleteDocument(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.DeleteDocument(uint(id)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// apiAddDocumentToDraft — добавляет документ в черновую заявку текущего
// пользователя; если черновика нет, создаёт его. ТЗ 4.1.13.
func (h *Handler) apiAddDocumentToDraft(ctx *gin.Context) {
	var req struct {
		DocumentID uint `json:"document_id"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil || req.DocumentID == 0 {
		h.errorHandler(ctx, 400, fmt.Errorf("document_id required"))
		return
	}
	userID := userctx.CurrentUserID()
	app, err := h.Repository.GetOrCreateDraftAccessRequest(userID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	if err := h.Repository.AddDocumentToAccessRequest(app.AccessRequestID, req.DocumentID); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	doc, err := h.Repository.GetDocumentByID(req.DocumentID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusOK, doc)
}

// apiAddDocumentToRequest — добавляет документ в указанную заявку. ТЗ 4.1.12.
func (h *Handler) apiAddDocumentToRequest(ctx *gin.Context) {
	reqID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	docID, err := strconv.Atoi(ctx.Param("documentId"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var body struct {
		AccessLevel float64 `json:"access_level"`
	}
	_ = ctx.ShouldBindJSON(&body)
	if err := h.Repository.AddDocumentToAccessRequest(uint(reqID), uint(docID)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	if body.AccessLevel > 0 {
		_ = h.Repository.UpdateRequestDocumentAccessLevel(uint(reqID), uint(docID), body.AccessLevel)
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "added"})
}

// Загрузка/замена изображения документа в MinIO. ТЗ 4.1.11.
func (h *Handler) apiUploadDocumentImage(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	defer file.Close()

	cli, cfg, err := storage.Client()
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// удаляем старый объект, если был
	svc, err := h.Repository.GetDocumentByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	if oldObj := storage.ObjectNameFromURL(cfg, svc.ImageURL); oldObj != "" {
		_ = cli.RemoveObject(ctx, cfg.Bucket, oldObj, minio.RemoveObjectOptions{})
	}

	objectName := fmt.Sprintf("document_%d_%d%s", id, time.Now().Unix(), filepath.Ext(header.Filename))
	uploadInfo, err := cli.PutObject(ctx, cfg.Bucket, objectName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	imageURL := storage.ObjectURL(cfg, uploadInfo.Key)
	if err := h.Repository.UpdateDocumentImage(uint(id), imageURL); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"image_url": imageURL})
}

// ===== Cart icon =====
func (h *Handler) apiGetCartIcon(ctx *gin.Context) {
	userID := userctx.CurrentUserID()
	app, err := h.Repository.GetDraftAccessRequest(userID)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"request_id": nil, "cart_count": 0})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"request_id": app.AccessRequestID, "cart_count": len(app.Documents)})
}

// ===== AccessRequests =====
func (h *Handler) apiListAccessRequests(ctx *gin.Context) {
	status := ctx.Query("status")
	fromStr := ctx.Query("from")
	toStr := ctx.Query("to")

	var fromPtr, toPtr *time.Time
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			fromPtr = &t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			toPtr = &t
		}
	}

	apps, err := h.Repository.ListAccessRequestsFiltered(status, fromPtr, toPtr)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusOK, apps)
}

func (h *Handler) apiGetAccessRequest(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	app, err := h.Repository.GetAccessRequestByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	ctx.JSON(http.StatusOK, app)
}

type accessRequestPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (h *Handler) apiUpdateAccessRequest(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var req accessRequestPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	req.Title = sanitizeString(req.Title)
	req.Description = sanitizeString(req.Description)
	if err := h.Repository.UpdateAccessRequestFields(uint(id), req.Title, req.Description); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) apiSubmitAccessRequest(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.MarkAccessRequestSubmitted(uint(id), userctx.CurrentUserID()); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiCompleteAccessRequest(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.MarkAccessRequestCompleted(uint(id), userctx.CurrentModeratorID()); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiRejectAccessRequest(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.MarkAccessRequestRejected(uint(id), userctx.CurrentModeratorID()); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiDeleteAccessRequestAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	// Ограничиваем удаление: только черновик и только создатель
	app, err := h.Repository.GetAccessRequestByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	if app.Status != repository.StatusDraft || app.CreatorID != userctx.CurrentUserID() {
		h.errorHandler(ctx, 400, fmt.Errorf("delete allowed only for own draft"))
		return
	}
	if err := h.Repository.DeleteAccessRequestByID(uint(id)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ===== m2m =====
type requestDocumentPayload struct {
	Quantity  int  `json:"quantity"`
	Position  int  `json:"position"`
	IsPrimary bool `json:"is_primary"`
}

func (h *Handler) apiUpdateRequestDocument(ctx *gin.Context) {
	appID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	documentID, err := strconv.Atoi(ctx.Param("documentId"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var req requestDocumentPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if err := h.Repository.UpdateRequestDocumentWithMeta(uint(appID), uint(documentID), req.Quantity, req.Position, req.IsPrimary); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiDeleteRequestDocument(ctx *gin.Context) {
	appID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	documentID, err := strconv.Atoi(ctx.Param("documentId"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.DeleteRequestDocument(uint(appID), uint(documentID)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ===== Users =====
type userPayload struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) apiRegisterUser(ctx *gin.Context) {
	var req userPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	u, err := h.Repository.CreateUser(req.Username, req.Email, req.Password, "user", false)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"id": u.UserID, "username": u.Username, "email": u.Email})
}

func (h *Handler) apiLogin(ctx *gin.Context) {
	var req userPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	user, err := h.Repository.GetUserByEmail(req.Email)
	if err != nil || user.PasswordHash != req.Password {
		h.errorHandler(ctx, 401, fmt.Errorf("invalid credentials"))
		return
	}
	tokenString, err := h.createToken(user.UserID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.SetCookie("token", tokenString, 86400, "/", "localhost", false, true)
	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *Handler) apiLogout(ctx *gin.Context) {
	ctx.SetCookie("token", "", -1, "/", "localhost", false, true)
	ctx.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) apiGetProfile(ctx *gin.Context) {
	userID := userctx.CurrentUserID()
	user, err := h.Repository.GetUserByID(userID)
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"id":       user.UserID,
		"email":    user.Email,
		"username": user.Username,
		"role":     user.Role,
	})
}

func (h *Handler) apiUpdateProfile(ctx *gin.Context) {
	userID := userctx.CurrentUserID()
	var req userPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.UpdateUser(userID, req.Username, req.Email, req.Password); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusOK)
}
