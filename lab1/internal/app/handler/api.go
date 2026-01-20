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

	// Services
	api.GET("/services", h.apiGetServices)
	api.GET("/services/:id", h.apiGetService)
	api.POST("/services", h.apiCreateService)
	api.PUT("/services/:id", h.apiUpdateService)
	api.DELETE("/services/:id", h.apiDeleteService)
	api.POST("/services/:id/add-to-cart", h.apiAddServiceToDraft)
	api.POST("/services/:id/image", h.apiUploadServiceImage)

	// Cart
	api.GET("/cart", h.apiGetCartIcon)

	// Applications
	api.GET("/applications", h.apiListApplications)
	api.GET("/applications/:id", h.apiGetApplication)
	api.PUT("/applications/:id", h.apiUpdateApplication)
	api.PUT("/applications/:id/submit", h.apiSubmitApplication)
	api.PUT("/applications/:id/complete", h.apiCompleteApplication)
	api.PUT("/applications/:id/reject", h.apiRejectApplication)
	api.DELETE("/applications/:id", h.apiDeleteApplicationAPI)

	// Application services (m-m)
	api.PUT("/applications/:id/services/:serviceId", h.apiUpdateApplicationService)
	api.DELETE("/applications/:id/services/:serviceId", h.apiDeleteApplicationService)

	// Users
	api.POST("/auth/register", h.apiRegisterUser)
	api.POST("/auth/login", h.apiLogin)
	api.POST("/auth/logout", h.apiLogout)
	api.GET("/profile", h.apiGetProfile)
	api.PUT("/profile", h.apiUpdateProfile)
}

// ===== Services =====
func (h *Handler) apiGetServices(ctx *gin.Context) {
	status := ctx.Query("status")
	name := ctx.Query("q")

	services, err := h.Repository.GetServicesFiltered(name, status)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusOK, services)
}

func (h *Handler) apiGetService(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	svc, err := h.Repository.GetServiceByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	ctx.JSON(http.StatusOK, svc)
}

type servicePayload struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ImageURL    string `json:"image_url"`
}

func (h *Handler) apiCreateService(ctx *gin.Context) {
	var req servicePayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	req.Title = sanitizeString(req.Title)
	req.Type = sanitizeString(req.Type)
	req.Description = sanitizeString(req.Description)
	req.Status = sanitizeString(req.Status)
	svc := repository.Service{
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
	created, err := h.Repository.CreateService(svc)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusCreated, created)
}

func (h *Handler) apiUpdateService(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var req servicePayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	req.Title = sanitizeString(req.Title)
	req.Type = sanitizeString(req.Type)
	req.Description = sanitizeString(req.Description)
	req.Status = sanitizeString(req.Status)
	if err := h.Repository.UpdateService(uint(id), req.Title, req.Type, req.Description, req.Status); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) apiDeleteService(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.DeleteService(uint(id)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) apiAddServiceToDraft(ctx *gin.Context) {
	serviceID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	userID := userctx.CurrentUserID()
	app, err := h.Repository.GetOrCreateDraftApplication(userID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	if err := h.Repository.AddServiceToApplication(app.ApplicationID, uint(serviceID)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"application_id": app.ApplicationID})
}

// Загрузка/замена изображения услуги (упрощённо: сохраняем локально в uploads/)
func (h *Handler) apiUploadServiceImage(ctx *gin.Context) {
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
	svc, err := h.Repository.GetServiceByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	if oldObj := storage.ObjectNameFromURL(cfg, svc.ImageURL); oldObj != "" {
		_ = cli.RemoveObject(ctx, cfg.Bucket, oldObj, minio.RemoveObjectOptions{})
	}

	objectName := fmt.Sprintf("service_%d_%d%s", id, time.Now().Unix(), filepath.Ext(header.Filename))
	uploadInfo, err := cli.PutObject(ctx, cfg.Bucket, objectName, file, header.Size, minio.PutObjectOptions{
		ContentType: header.Header.Get("Content-Type"),
	})
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	imageURL := storage.ObjectURL(cfg, uploadInfo.Key)
	if err := h.Repository.UpdateServiceImage(uint(id), imageURL); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"image_url": imageURL})
}

// ===== Cart icon =====
func (h *Handler) apiGetCartIcon(ctx *gin.Context) {
	userID := userctx.CurrentUserID()
	app, err := h.Repository.GetDraftApplication(userID)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"application_id": nil, "count": 0})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"application_id": app.ApplicationID, "count": len(app.Services)})
}

// ===== Applications =====
func (h *Handler) apiListApplications(ctx *gin.Context) {
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

	apps, err := h.Repository.ListApplicationsFiltered(status, fromPtr, toPtr)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.JSON(http.StatusOK, apps)
}

func (h *Handler) apiGetApplication(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	app, err := h.Repository.GetApplicationByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	ctx.JSON(http.StatusOK, app)
}

type applicationPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (h *Handler) apiUpdateApplication(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var req applicationPayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	req.Title = sanitizeString(req.Title)
	req.Description = sanitizeString(req.Description)
	if err := h.Repository.UpdateApplicationFields(uint(id), req.Title, req.Description); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) apiSubmitApplication(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.MarkApplicationSubmitted(uint(id), userctx.CurrentUserID()); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiCompleteApplication(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.MarkApplicationCompleted(uint(id), userctx.CurrentModeratorID()); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiRejectApplication(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.MarkApplicationRejected(uint(id), userctx.CurrentModeratorID()); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiDeleteApplicationAPI(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	// Ограничиваем удаление: только черновик и только создатель
	app, err := h.Repository.GetApplicationByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}
	if app.Status != repository.StatusDraft || app.CreatorID != userctx.CurrentUserID() {
		h.errorHandler(ctx, 400, fmt.Errorf("delete allowed only for own draft"))
		return
	}
	if err := h.Repository.DeleteApplicationByID(uint(id)); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ===== m2m =====
type appServicePayload struct {
	Quantity  int  `json:"quantity"`
	Position  int  `json:"position"`
	IsPrimary bool `json:"is_primary"`
}

func (h *Handler) apiUpdateApplicationService(ctx *gin.Context) {
	appID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	serviceID, err := strconv.Atoi(ctx.Param("serviceId"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	var req appServicePayload
	if err := ctx.ShouldBindJSON(&req); err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if err := h.Repository.UpdateApplicationServiceWithMeta(uint(appID), uint(serviceID), req.Quantity, req.Position, req.IsPrimary); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}
	ctx.Status(http.StatusOK)
}

func (h *Handler) apiDeleteApplicationService(ctx *gin.Context) {
	appID, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	serviceID, err := strconv.Atoi(ctx.Param("serviceId"))
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	if err := h.Repository.DeleteApplicationService(uint(appID), uint(serviceID)); err != nil {
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
