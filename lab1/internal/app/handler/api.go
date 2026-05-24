package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"

	"lab1/internal/app/auth"
	"lab1/internal/app/middleware"
	"lab1/internal/app/repository"
	"lab1/internal/app/storage"
	"lab1/internal/app/userctx"
)

// currentUser — достаёт user_id из контекста (JWT middleware), иначе берёт singleton.
// Это позволяет хендлерам работать и в обычном режиме (lab3-style singleton),
// и под авторизацией (lab4-style JWT).
func currentUser(ctx *gin.Context) uint {
	if v, ok := ctx.Get("user_id"); ok {
		if id, ok := v.(uint); ok && id > 0 {
			return id
		}
	}
	return userctx.CurrentUserID()
}

// RegisterAPI — раскладывает endpoints по группам доступа.
// Гостям доступны только чтение и аутентификация.
// Авторизованным — операции создателя заявки.
// Модератору — управление документами и одобрение заявок.
func (h *Handler) RegisterAPI(router *gin.Engine) {
	api := router.Group("/api")

	// ===== Гости (без токена) =====
	api.POST("/sign_up", h.apiRegisterUser)
	api.POST("/login", h.apiLogin)
	api.GET("/documents", h.apiGetDocuments)
	api.GET("/documents/:id", h.apiGetDocument)

	// ===== Авторизованные пользователи =====
	authed := api.Group("/", middleware.RequireAuth())
	{
		authed.POST("/logout", h.apiLogout)
		authed.GET("/profile", h.apiGetProfile)
		authed.PUT("/profile", h.apiUpdateProfile)

		// Корзина и заявки текущего пользователя
		authed.GET("/cart-icon", h.apiGetCartIcon)
		authed.GET("/document-requests", h.apiListAccessRequests)
		authed.GET("/document-requests/:id", h.apiGetAccessRequest)
		authed.PUT("/document-requests/:id", h.apiUpdateAccessRequest)
		authed.PUT("/document-requests/:id/form", h.apiSubmitAccessRequest)
		authed.DELETE("/document-requests/:id", h.apiDeleteAccessRequestAPI)

		// м-м: добавить/обновить/удалить документ в заявке
		authed.POST("/document-requests/add-to-draft", h.apiAddDocumentToDraft)
		authed.POST("/document-requests/:id/documents/:documentId", h.apiAddDocumentToRequest)
		authed.PUT("/document-requests/:id/documents/:documentId", h.apiUpdateRequestDocument)
		authed.DELETE("/document-requests/:id/documents/:documentId", h.apiDeleteRequestDocument)
	}

	// ===== Модератор =====
	mod := api.Group("/", middleware.RequireAuth(), middleware.RequireRole(middleware.RoleModerator))
	{
		// CRUD документов — только модератор
		mod.POST("/documents", h.apiCreateDocument)
		mod.PUT("/documents/:id", h.apiUpdateDocument)
		mod.DELETE("/documents/:id", h.apiDeleteDocument)
		mod.POST("/documents/:id/image", h.apiUploadDocumentImage)

		// Завершить/отклонить заявку — только модератор
		mod.PUT("/document-requests/:id/complete", h.apiCompleteAccessRequest)
		mod.PUT("/document-requests/:id/reject", h.apiRejectAccessRequest)
	}
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
		CreatorID:   currentUser(ctx),
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
	userID := currentUser(ctx)
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
	userID := currentUser(ctx)
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
	if err := h.Repository.MarkAccessRequestSubmitted(uint(id), currentUser(ctx)); err != nil {
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
	if err := h.Repository.MarkAccessRequestCompleted(uint(id), currentUser(ctx)); err != nil {
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
	if err := h.Repository.MarkAccessRequestRejected(uint(id), currentUser(ctx)); err != nil {
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
	if app.Status != repository.StatusDraft || app.CreatorID != currentUser(ctx) {
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

// apiLogin — выпускает JWT с role внутри claims; кладёт в cookie И возвращает
// access_token в теле ответа (по ТЗ 4.1.2).
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
	role := middleware.RoleUser
	if user.IsModerator {
		role = middleware.RoleModerator
	}
	token, _, expiresIn, err := auth.GenerateToken(user.UserID, role)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.SetCookie("token", token, int(expiresIn), "/", "localhost", false, true)
	ctx.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   expiresIn,
	})
}

// apiLogout — отзывает текущий jti через Redis blacklist (по ТЗ 4.1.3).
// Cookie тоже очищается, чтобы браузер не присылал старый токен.
func (h *Handler) apiLogout(ctx *gin.Context) {
	if v, ok := ctx.Get("jti"); ok {
		if jti, ok := v.(string); ok && jti != "" {
			// TTL берём из exp токена в контексте; если не нашли — 24 часа
			ttl := int64(24 * 60 * 60)
			_ = auth.BlacklistJTI(ctx.Request.Context(), jti, ttl)
		}
	}
	ctx.SetCookie("token", "", -1, "/", "localhost", false, true)
	ctx.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) apiGetProfile(ctx *gin.Context) {
	userID := currentUser(ctx)
	user, err := h.Repository.GetUserByID(userID)
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	role := 0
	if user.IsModerator {
		role = 1
	}
	ctx.JSON(http.StatusOK, gin.H{
		"uuid":  fmt.Sprintf("%d", user.UserID),
		"login": user.Username,
		"role":  role,
	})
}

func (h *Handler) apiUpdateProfile(ctx *gin.Context) {
	userID := currentUser(ctx)
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
