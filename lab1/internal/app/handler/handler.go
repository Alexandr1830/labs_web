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
	router.GET("/", h.GetOrders)
	router.GET("/order/:id", h.GetOrder)
	router.POST("/login", h.LoginUser)

	// закрытые маршруты
	auth := router.Group("/")
	auth.Use(middleware.JWTMiddleware())
	{
		router.POST("/application/add/:serviceId", middleware.JWTMiddleware(), h.AddService)
		router.POST("/application/:id/service/:serviceId/access", middleware.JWTMiddleware(), h.UpdateServiceAccess)
		router.POST("/application/:id/delete", middleware.JWTMiddleware(), h.DeleteApplication)
		router.GET("/application/:id", middleware.JWTMiddleware(), h.GetApplication)
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
func (h *Handler) GetOrders(ctx *gin.Context) {
	query := ctx.Query("query")

	var (
		services []repository.Service
		err      error
	)

	if query == "" {
		services, err = h.Repository.GetAllServices()
	} else {
		services, err = h.Repository.GetServicesByName(query)
	}

	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Черновик создаём только при добавлении услуги
	appCount := 0
	var appID uint
	if token, err := ctx.Cookie("token"); err == nil && token != "" {
		if parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte("your-secret-key"), nil
		}); err == nil && parsed.Valid {
			if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
				if uid, ok := claims["user_id"].(float64); ok {
					if app, err := h.Repository.GetDraftApplication(uint(uid)); err == nil {
						appCount = len(app.Services)
						appID = app.ApplicationID
					}
				}
			}
		}
	}

	ctx.HTML(http.StatusOK, "index.html", gin.H{
		"orders":   services,
		"query":    query,
		"AppCount": appCount,
		"AppID":    appID,
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

	doc, err := h.Repository.GetServiceByID(uint(id))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	ctx.HTML(http.StatusOK, "order.html", doc)
}

// ========== ДОБАВИТЬ УСЛУГУ В ЗАЯВКУ ==========
func (h *Handler) AddService(ctx *gin.Context) {

	serviceIDStr := ctx.Param("serviceId")
	serviceID, err := strconv.Atoi(serviceIDStr)
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

	// Проверяем услугу
	_, err = h.Repository.GetServiceByID(uint(serviceID))
	if err != nil {
		h.errorHandler(ctx, 404, err)
		return
	}

	// Единственная заявка
	app, err := h.Repository.GetOrCreateDraftApplication(userID)
	if err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	// Добавляем услугу
	err = h.Repository.AddServiceToApplication(app.ApplicationID, uint(serviceID))
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

	// Переход на главную
	ctx.Redirect(http.StatusSeeOther, "/")
}

// ========== ОБНОВИТЬ УРОВЕНЬ ДОСТУПА УСЛУГИ В ЗАЯВКЕ ==========
func (h *Handler) UpdateServiceAccess(ctx *gin.Context) {
	appIDStr := ctx.Param("id")
	serviceIDStr := ctx.Param("serviceId")
	levelStr := ctx.PostForm("access_level")

	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	serviceID, err := strconv.Atoi(serviceIDStr)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}
	level, err := strconv.ParseFloat(levelStr, 64)
	if err != nil {
		h.errorHandler(ctx, 400, err)
		return
	}

	if err := h.Repository.UpdateServiceAccessLevel(uint(appID), uint(serviceID), level); err != nil {
		h.errorHandler(ctx, 500, err)
		return
	}

	ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/application/%d", appID))
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
