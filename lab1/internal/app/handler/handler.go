package handler

import (
	"fmt"
	"html/template"
	"lab1/internal/app/middleware"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lab1/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

// ========== РЕГИСТРАЦИЯ МАРШРУТОВ ==========
func (h *Handler) RegisterHandler(router *gin.Engine) {

	// открытые маршруты
	router.GET("/", h.GetDocuments)
	router.GET("/document/:id", h.GetDocument)
	router.POST("/login", h.LoginUser)

	// закрытые маршруты
	auth := router.Group("/")
	auth.Use(middleware.JWTMiddleware())
	{
		router.POST("/document-request/add/:documentId", middleware.JWTMiddleware(), h.AddDocument)
		router.POST("/document-request/:id/document/:documentId/access", middleware.JWTMiddleware(), h.UpdateDocumentAccess)
		router.POST("/document-request/:id/delete", middleware.JWTMiddleware(), h.DeleteAccessRequest)
		router.GET("/document-request/:id", middleware.JWTMiddleware(), h.GetAccessRequest)
	}
}

// ========== РЕГИСТРАЦИЯ СТАТИКИ ==========
func (h *Handler) RegisterStatic(router *gin.Engine) {

	resolveDir := func(options []string) string {
		for _, dir := range options {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
		return options[len(options)-1]
	}

	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			months := map[time.Month]string{
				time.January: "янв.", time.February: "февр.", time.March: "мар.",
				time.April: "апр.", time.May: "мая", time.June: "июн.",
				time.July: "июл.", time.August: "авг.", time.September: "сент.",
				time.October: "окт.", time.November: "ноя.", time.December: "дек.",
			}
			return fmt.Sprintf("%02d %s %d", t.Day(), months[t.Month()], t.Year())
		},

		"trim": func(s, prefix, suffix string) string {
			s = strings.TrimPrefix(s, prefix)
			s = strings.TrimSuffix(s, suffix)
			s = strings.ReplaceAll(s, "\"", "")
			s = strings.ReplaceAll(s, ",", ", ")
			return s
		},
	}

	router.SetFuncMap(funcMap)

	templateDir := resolveDir([]string{"templates", "../../templates"})
	router.LoadHTMLGlob(filepath.Join(templateDir, "*"))

	resourceDir := resolveDir([]string{"resources", "../../resources"})
	router.Static("/resources", resourceDir)
}

// ========== ERROR ==========
func (h *Handler) errorHandler(ctx *gin.Context, code int, err error) {
	logrus.Error(err.Error())
	ctx.JSON(code, gin.H{
		"status":      "error",
		"description": err.Error(),
	})
}

// ========== СТРАНИЦА ДОКУМЕНТОВ ==========
func (h *Handler) GetDocuments(ctx *gin.Context) {
	query := ctx.Query("query")

	var (
		documents []repository.Document
		err       error
	)

	if query == "" {
		documents, err = h.Repository.GetAllDocuments()
	} else {
		documents, err = h.Repository.GetDocumentsByName(query)
	}

	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Черновик создаём только при добавлении документа
	appCount := 0
	var appID uint
	if token, err := ctx.Cookie("token"); err == nil && token != "" {
		if parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte("your-secret-key"), nil
		}); err == nil && parsed.Valid {
			if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
				if uid, ok := claims["user_id"].(float64); ok {
					if app, err := h.Repository.GetDraftAccessRequest(uint(uid)); err == nil {
						appCount = len(app.Documents)
						appID = app.AccessRequestID
					}
				}
			}
		}
	}

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"documents": documents,
		"query":     query,
		"AppCount":  appCount,
		"AppID":     appID,
	})
}

// ========== ОДИН ДОКУМЕНТ ==========
func (h *Handler) GetDocument(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	doc, err := h.Repository.GetDocumentByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	ctx.HTML(http.StatusOK, "document.html", doc)
}

// ========== ДОБАВИТЬ ДОКУМЕНТ В ЗАЯВКУ ==========
func (h *Handler) AddDocument(ctx *gin.Context) {

	documentIDStr := ctx.Param("documentId")
	documentID, err := strconv.Atoi(documentIDStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	uid, ok := ctx.Get("user_id")
	if !ok {
		h.errorHandler(ctx, 401, fmt.Errorf("missing user"))
		return
	}
	userID := uid.(uint)

	// Проверяем документ
	_, err = h.Repository.GetDocumentByID(uint(documentID))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	// Единственная заявка
	app, err := h.Repository.GetOrCreateDraftAccessRequest(userID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Добавляем документ
	err = h.Repository.AddDocumentToAccessRequest(app.AccessRequestID, uint(documentID))
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Переходим к заявке
	ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/document-request/%d", app.AccessRequestID))
}

// ========== СТРАНИЦА ЗАЯВКИ ==========
func (h *Handler) GetAccessRequest(ctx *gin.Context) {

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	app, err := h.Repository.GetAccessRequestByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	ctx.HTML(http.StatusOK, "request.html", app)
}

// ========== УДАЛИТЬ ЗАЯВКУ ==========

func (h *Handler) DeleteAccessRequest(ctx *gin.Context) {

	idStr := ctx.Param("id")
	id, _ := strconv.Atoi(idStr)

	// Помечаем заявку как удалённую
	err := h.Repository.DeleteAccessRequestByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Переход на главную
	ctx.Redirect(http.StatusSeeOther, "/")
}

// ========== ОБНОВИТЬ УРОВЕНЬ ДОСТУПА ДОКУМЕНТА В ЗАЯВКЕ ==========
func (h *Handler) UpdateDocumentAccess(ctx *gin.Context) {
	appIDStr := ctx.Param("id")
	documentIDStr := ctx.Param("documentId")
	levelStr := ctx.PostForm("access_level")

	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	documentID, err := strconv.Atoi(documentIDStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	level, err := strconv.ParseFloat(levelStr, 64)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	if err := h.Repository.UpdateRequestDocumentAccessLevel(uint(appID), uint(documentID), level); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/document-request/%d", appID))
}

func (h *Handler) LoginUser(ctx *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	user, err := h.Repository.GetUserByEmail(req.Email)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.PasswordHash != req.Password {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// === СОЗДАЁМ JWT ТОКЕН ===
	tokenStruct := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.UserID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := tokenStruct.SignedString([]byte("your-secret-key"))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	// === СТАВИМ COOKIE С JWT ===
	ctx.SetCookie(
		"token",     // имя cookie
		tokenString, // jwt
		86400,       // 1 день
		"/",         // путь
		"localhost", // домен
		false,       // secure=false (локалка)
		true,        // HttpOnly=true — ОБЯЗАТЕЛЬНО!!!
	)

	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *Handler) createToken(uid uint) (string, error) {
	tokenStruct := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": uid,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	return tokenStruct.SignedString([]byte("your-secret-key"))
}
