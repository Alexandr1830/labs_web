package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
    "lab1/internal/app/middleware"

	"lab1/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/golang-jwt/jwt/v5"
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
	router.GET("/", h.GetOrders)
	router.GET("/order/:id", h.GetOrder)
	router.POST("/login", h.LoginUser)

	// закрытые маршруты
	auth := router.Group("/")
	auth.Use(middleware.JWTMiddleware())
	{
		router.POST("/application/add/:docId", middleware.JWTMiddleware(), h.AddDocument)
		router.POST("/application/:id/delete", middleware.JWTMiddleware(), h.DeleteApplication)
		router.GET("/application/:id", middleware.JWTMiddleware(), h.GetApplication)
	}
}

// ========== РЕГИСТРАЦИЯ СТАТИКИ ==========
func (h *Handler) RegisterStatic(router *gin.Engine) {

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
	router.LoadHTMLGlob("templates/*")
	router.Static("/resources", "./resources")
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
func (h *Handler) GetOrders(ctx *gin.Context) {
	query := ctx.Query("query")

	var (
		docs []repository.Document
		err  error
	)

	if query == "" {
		docs, err = h.Repository.GetAllDocuments()
	} else {
		docs, err = h.Repository.GetDocumentsByTitle(query)
	}

	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Кол-во документов в единственной заявке
	app, _ := h.Repository.GetOrCreateDraftApplication()

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"orders":   docs,
		"query":    query,
		"AppCount": len(app.Documents),
		"AppID":    app.ApplicationID,
	})
}

// ========== ОДИН ДОКУМЕНТ ==========
func (h *Handler) GetOrder(ctx *gin.Context) {
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

	ctx.HTML(http.StatusOK, "order.html", doc)
}

// ========== ДОБАВИТЬ ДОКУМЕНТ В ЗАЯВКУ ==========
func (h *Handler) AddDocument(ctx *gin.Context) {

	docIdStr := ctx.Param("docId")
	docId, err := strconv.Atoi(docIdStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	// Документ
	doc, err := h.Repository.GetDocumentByID(uint(docId))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	// Единственная заявка
	app, err := h.Repository.GetOrCreateDraftApplication()
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Добавляем документ
	err = h.Repository.AddDocumentToApplication(app.ApplicationID, doc.DocumentID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Переходим к заявке
	ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/application/%d", app.ApplicationID))
}

// ========== СТРАНИЦА ЗАЯВКИ ==========
func (h *Handler) GetApplication(ctx *gin.Context) {

	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	app, err := h.Repository.GetApplicationByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	ctx.HTML(http.StatusOK, "application.html", app)
}

// ========== УДАЛИТЬ ЗАЯВКУ ==========

func (h *Handler) DeleteApplication(ctx *gin.Context) {

	idStr := ctx.Param("id")
	id, _ := strconv.Atoi(idStr)

	// Помечаем заявку как удалённую
	err := h.Repository.DeleteApplicationByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Создаём новую "пустую" заявку
	_, _ = h.Repository.GetOrCreateDraftApplication()

	// Переход на главную
	ctx.Redirect(http.StatusSeeOther, "/")
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
        "token",        // имя cookie
        tokenString,    // jwt
        86400,          // 1 день
        "/",            // путь
        "localhost",    // домен
        false,          // secure=false (локалка)
        true,           // HttpOnly=true — ОБЯЗАТЕЛЬНО!!!
    )

    ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}