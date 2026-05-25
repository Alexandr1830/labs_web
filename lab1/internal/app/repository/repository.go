package repository

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	StatusDraft     = "draft"
	StatusDeleted   = "deleted"
	StatusFormed    = "formed"
	StatusCompleted = "completed"
	StatusRejected  = "rejected"
)

type Repository struct {
	db *gorm.DB
}

//
// ========== МОДЕЛИ ==========
//

type User struct {
	UserID       uint      `gorm:"primaryKey;column:user_id"`
	Username     string    `gorm:"type:varchar(100);not null"`
	Email        string    `gorm:"type:varchar(255);unique;not null"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	Role         string    `gorm:"type:varchar(50);not null"`
	IsModerator  bool      `gorm:"column:is_moderator;default:false"`
	DateJoined   time.Time `gorm:"autoCreateTime"`
}

type Document struct {
	DocumentID  uint      `gorm:"primaryKey;column:document_id"`
	Title       string    `gorm:"type:varchar(255);not null;column:name"`
	Type        string    `gorm:"type:varchar(50);not null;column:type"`
	Description string    `gorm:"type:text"`
	Status      string    `gorm:"type:varchar(50);not null;default:'active'"`
	ImageURL    string    `gorm:"type:varchar(255);column:image_url"`
	AccessLevel float64   `gorm:"type:numeric(12,2);default:1;column:access_level"` // 1=чтение, 2=запись, 3=чтение+запись
	IsDeleted   bool      `gorm:"column:is_deleted;default:false"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
	CreatorID   uint      `gorm:"column:creator_id"`
	Creator     User      `gorm:"foreignKey:CreatorID;references:UserID"`
}

func (Document) TableName() string { return "documents" }

// Единственная заявка (корзина)
type AccessRequest struct {
	AccessRequestID uint       `gorm:"primaryKey;column:access_request_id"`
	Title           string     `gorm:"column:title"`
	Description     string     `gorm:"column:description"`
	Status          string     `gorm:"column:status"`
	IsDeleted       bool       `gorm:"column:is_deleted;default:false"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	FormedAt        *time.Time `gorm:"column:formed_at"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
	CreatorID       uint       `gorm:"column:creator_id"`
	ModeratorID     *uint      `gorm:"column:moderator_id"`
	TotalAccess     float64    `gorm:"column:total_access;type:numeric(12,2);default:0"` // в колонке total_access храним суммарный коэффициент доступа
	DeliveryDate    *time.Time `gorm:"column:delivery_date"`

	Documents []Document `gorm:"many2many:access_request_documents;joinForeignKey:AccessRequestID;joinReferences:DocumentID"`
	Creator   User       `gorm:"foreignKey:CreatorID;references:UserID"`
	Moderator *User      `gorm:"foreignKey:ModeratorID;references:UserID"`

	ComputedResult int `gorm:"-"`
}

func (AccessRequest) TableName() string { return "access_requests" }

type RequestDocument struct {
	AccessRequestID uint    `gorm:"primaryKey;column:access_request_id"`
	DocumentID      uint    `gorm:"primaryKey;column:document_id"`
	Quantity        int     `gorm:"column:quantity;default:1"`
	Position        int     `gorm:"column:position;default:0"`
	IsPrimary       bool    `gorm:"column:is_primary;default:false"`
	LineTotal       float64 `gorm:"column:line_total;type:numeric(12,2);default:0"`
	AccessLevel     float64 `gorm:"column:access_level;type:numeric(12,2);default:1"`
}

func (RequestDocument) TableName() string {
	return "access_request_documents"
}

//
// ========== ИНИЦИАЛИЗАЦИЯ ENV ==========
//

func init() {
	// .env может лежать в lab1/ или в lab1/cmd/<binary>/ — поэтому
	// пробуем несколько кандидатов, первый существующий выигрывает.
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(p); err == nil {
			break
		}
	}

	fmt.Println("DB_HOST =", os.Getenv("DB_HOST"))
	fmt.Println("DB_PORT =", os.Getenv("DB_PORT"))
	fmt.Println("DB_USER =", os.Getenv("DB_USER"))
	fmt.Println("DB_NAME =", os.Getenv("DB_NAME"))
}

//
// ========== ПОДКЛЮЧЕНИЕ К БАЗЕ ==========
//

func New() (*Repository, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_NAME"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %v", err)
	}

	err = db.AutoMigrate(&User{}, &Document{}, &AccessRequest{}, &RequestDocument{})
	if err != nil {
		return nil, fmt.Errorf("ошибка миграции: %v", err)
	}

	fmt.Println("Подключение к PostgreSQL успешно")
	return &Repository{db: db}, nil
}

//
// ========== МЕТОДЫ DOCUMENTS ==========
//

func (r *Repository) GetAllDocuments() ([]Document, error) {
	var documents []Document
	result := r.db.
		Where("is_deleted = FALSE").
		Preload("Creator").
		Find(&documents)

	return documents, result.Error
}

func (r *Repository) GetDocumentsByName(name string) ([]Document, error) {
	var documents []Document
	result := r.db.
		Where("is_deleted = FALSE").
		Where("name ILIKE ?", "%"+name+"%").
		Preload("Creator").
		Find(&documents)

	return documents, result.Error
}

func (r *Repository) GetDocumentByID(id uint) (Document, error) {
	var document Document
	result := r.db.
		Where("is_deleted = FALSE").
		Preload("Creator").
		First(&document, id)

	return document, result.Error
}

// Фильтр документов по имени и статусу
func (r *Repository) GetDocumentsFiltered(name, status string) ([]Document, error) {
	var documents []Document
	q := r.db.Where("is_deleted = FALSE")
	if name != "" {
		q = q.Where("name ILIKE ?", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Preload("Creator").Find(&documents).Error; err != nil {
		return nil, err
	}
	return documents, nil
}

func (r *Repository) CreateDocument(s Document) (Document, error) {
	if s.Status == "" {
		s.Status = "active"
	}
	if err := r.db.Create(&s).Error; err != nil {
		return Document{}, err
	}
	return s, nil
}

func (r *Repository) UpdateDocument(id uint, title, typ, desc, status string) error {
	fields := map[string]interface{}{
		"name":        title,
		"type":        typ,
		"description": desc,
	}
	if status != "" {
		fields["status"] = status
	}
	return r.db.Model(&Document{}).
		Where("document_id = ? AND is_deleted = FALSE", id).
		Updates(fields).Error
}

func (r *Repository) DeleteDocument(id uint) error {
	return r.db.Model(&Document{}).
		Where("document_id = ?", id).
		Update("is_deleted", true).Error
}

// Обновить ссылку на изображение документа, удаляя старый путь при необходимости.
func (r *Repository) UpdateDocumentImage(id uint, imageURL string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var svc Document
		if err := tx.First(&svc, id).Error; err != nil {
			return err
		}
		// Физическое удаление файла, если путь локальный и отличается
		if svc.ImageURL != "" && svc.ImageURL != imageURL && strings.HasPrefix(svc.ImageURL, "uploads/") {
			_ = os.Remove(svc.ImageURL)
		}
		return tx.Model(&Document{}).Where("document_id = ?", id).Update("image_url", imageURL).Error
	})
}

//
// ========== МЕТОДЫ ACCESS REQUEST (ОДНА ЗАЯВКА) ==========
//

// Найти или создать черновик конкретного пользователя
func (r *Repository) GetOrCreateDraftAccessRequest(userID uint) (AccessRequest, error) {
	var app AccessRequest

	result := r.db.
		Preload("Documents").
		Where("status = ? AND is_deleted = FALSE AND creator_id = ?", StatusDraft, userID).
		First(&app)

	if result.Error == nil {
		return app, nil
	}

	app = AccessRequest{
		Title:     "Заявка",
		Status:    StatusDraft,
		CreatorID: userID,
	}

	if err := r.db.Create(&app).Error; err != nil {
		return AccessRequest{}, err
	}

	return app, nil
}

func (r *Repository) GetDraftAccessRequest(userID uint) (AccessRequest, error) {
	var app AccessRequest
	err := r.db.
		Preload("Documents").
		Where("status = ? AND is_deleted = FALSE AND creator_id = ?", StatusDraft, userID).
		First(&app).Error
	return app, err
}

// Добавить услугу в заявку с расчётом суммы
func (r *Repository) AddDocumentToAccessRequest(appID, documentID uint) error {
	var app AccessRequest
	if err := r.db.Where("is_deleted = FALSE").First(&app, appID).Error; err != nil {
		return err
	}

	var document Document
	if err := r.db.Where("is_deleted = FALSE").First(&document, documentID).Error; err != nil {
		return err
	}

	var link RequestDocument
	err := r.db.First(&link, "access_request_id = ? AND document_id = ?", appID, documentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		link = RequestDocument{
			AccessRequestID: appID,
			DocumentID:      documentID,
			Quantity:        1,
			AccessLevel:     1,
			LineTotal:       computeLineTotal(1, 1, additionalFactor(document.Type)),
		}
		if createErr := r.db.Create(&link).Error; createErr != nil {
			return createErr
		}
	} else if err == nil {
		link.Quantity++
		link.LineTotal = computeLineTotal(link.AccessLevel, link.Quantity, additionalFactor(document.Type))
		if updErr := r.db.Model(&RequestDocument{}).
			Where("access_request_id = ? AND document_id = ?", appID, documentID).
			Updates(map[string]interface{}{"quantity": link.Quantity, "line_total": link.LineTotal}).Error; updErr != nil {
			return updErr
		}
	} else {
		return err
	}

	return r.recalcTotals(appID)
}

func (r *Repository) recalcTotals(appID uint) error {
	return r.recalcTotalsTx(r.db, appID)
}

func (r *Repository) recalcTotalsTx(db *gorm.DB, appID uint) error {
	var total float64
	if err := db.Model(&RequestDocument{}).
		Select("COALESCE(SUM(line_total),0)").
		Where("access_request_id = ?", appID).
		Scan(&total).Error; err != nil {
		return err
	}

	return db.Model(&AccessRequest{}).
		Where("access_request_id = ?", appID).
		Update("total_access", total).Error
}

// Создать пользователя (регистрация)
func (r *Repository) CreateUser(username, email, pass, role string, isModerator bool) (User, error) {
	u := User{
		Username:     username,
		Email:        email,
		PasswordHash: pass,
		Role:         role,
		IsModerator:  isModerator,
	}
	if role == "" {
		u.Role = "user"
	}
	if err := r.db.Create(&u).Error; err != nil {
		return User{}, err
	}
	return u, nil
}

// Обновить пользователя (профиль)
func (r *Repository) UpdateUser(id uint, username, email, pass string) error {
	fields := map[string]interface{}{
		"username": username,
		"email":    email,
	}
	if pass != "" {
		fields["password_hash"] = pass
	}
	return r.db.Model(&User{}).
		Where("user_id = ?", id).
		Updates(fields).Error
}

// Очистить заявку (полностью убрать все услуги)
func (r *Repository) ClearAccessRequest(appID uint) error {
	var app AccessRequest

	if err := r.db.First(&app, appID).Error; err != nil {
		return err
	}

	return r.db.Model(&app).Association("Documents").Clear()
}

func (r *Repository) GetAccessRequestByID(id uint) (AccessRequest, error) {
	var app AccessRequest

	result := r.db.
		Preload("Documents").
		Preload("Creator").
		Preload("Moderator").
		Where("is_deleted = FALSE").
		First(&app, id)

	if result.Error != nil {
		return app, result.Error
	}

	// Подтягиваем уровни доступа из m-m
	var links []RequestDocument
	if err := r.db.Where("access_request_id = ?", id).Find(&links).Error; err != nil {
		return app, err
	}
	linkMap := make(map[uint]RequestDocument)
	for _, l := range links {
		linkMap[l.DocumentID] = l
	}
	for i := range app.Documents {
		if l, ok := linkMap[app.Documents[i].DocumentID]; ok {
			app.Documents[i].AccessLevel = l.AccessLevel
		}
	}

	return app, nil
}

func (r *Repository) DeleteAccessRequestByID(id uint) error {
	// Логическое удаление через SQL UPDATE (без ORM)
	return r.db.Exec(
		"UPDATE access_requests SET status = ?, is_deleted = TRUE WHERE access_request_id = ?",
		StatusDeleted, id,
	).Error
}

// Обновить поля заявки
func (r *Repository) UpdateAccessRequestFields(id uint, title, desc string) error {
	fields := map[string]interface{}{
		"title":       title,
		"description": desc,
	}
	// Системные поля не трогаем
	return r.db.Model(&AccessRequest{}).
		Where("access_request_id = ? AND is_deleted = FALSE AND status IN ?", id, []string{StatusDraft, StatusFormed}).
		Updates(fields).Error
}

// Список заявок (без удалённых/черновиков) с фильтрами по статусу и дате формирования.
func (r *Repository) ListAccessRequestsFiltered(status string, from, to *time.Time) ([]AccessRequest, error) {
	var apps []AccessRequest
	q := r.db.
		Where("is_deleted = FALSE").
		Where("status <> ?", StatusDraft)

	if status != "" {
		q = q.Where("status = ?", status)
	}
	if from != nil {
		q = q.Where("formed_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("formed_at <= ?", *to)
	}

	err := q.
		Preload("Documents").
		Preload("Creator").
		Preload("Moderator").
		Find(&apps).Error
	if err != nil {
		return nil, err
	}

	// вычисляем количество услуг с ненулевым line_total как computed
	for i := range apps {
		var cnt int64
		if err := r.db.Model(&RequestDocument{}).
			Where("access_request_id = ? AND line_total > 0", apps[i].AccessRequestID).
			Count(&cnt).Error; err == nil {
			apps[i].ComputedResult = int(cnt)
		}
	}
	return apps, nil
}

func (r *Repository) MarkAccessRequestSubmitted(id uint, creatorID uint) error {
	// Только создатель может формировать черновик.
	return r.db.Model(&AccessRequest{}).
		Where("access_request_id = ? AND creator_id = ? AND status = ?", id, creatorID, StatusDraft).
		Updates(map[string]interface{}{
			"status":    StatusFormed,
			"formed_at": time.Now(),
		}).Error
}

func (r *Repository) MarkAccessRequestCompleted(id uint, moderatorID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AccessRequest{}).
			Where("access_request_id = ? AND status = ?", id, StatusFormed).
			Updates(map[string]interface{}{
				"status":        StatusCompleted,
				"completed_at":  time.Now(),
				"moderator_id":  moderatorID,
				"delivery_date": time.Now().AddDate(0, 1, 0),
			}).Error; err != nil {
			return err
		}
		return r.recalcTotalsTx(tx, id)
	})
}

func (r *Repository) MarkAccessRequestRejected(id uint, moderatorID uint) error {
	return r.db.Model(&AccessRequest{}).
		Where("access_request_id = ? AND status = ?", id, StatusFormed).
		Updates(map[string]interface{}{
			"status":       StatusRejected,
			"completed_at": time.Now(),
			"moderator_id": moderatorID,
		}).Error
}

func (r *Repository) UpdateRequestDocument(appID, documentID uint, qty int) error {
	if qty <= 0 {
		qty = 1
	}
	var svc Document
	if err := r.db.First(&svc, documentID).Error; err != nil {
		return err
	}
	var link RequestDocument
	if err := r.db.First(&link, "access_request_id = ? AND document_id = ?", appID, documentID).Error; err != nil {
		return err
	}
	line := computeLineTotal(link.AccessLevel, qty, additionalFactor(svc.Type))
	return r.db.Model(&RequestDocument{}).
		Where("access_request_id = ? AND document_id = ?", appID, documentID).
		Updates(map[string]interface{}{
			"quantity":   qty,
			"line_total": line,
			"position":   link.Position,
			"is_primary": link.IsPrimary,
		}).Error
}

func (r *Repository) UpdateRequestDocumentWithMeta(appID, documentID uint, qty int, pos int, primary bool) error {
	if qty <= 0 {
		qty = 1
	}
	var svc Document
	if err := r.db.First(&svc, documentID).Error; err != nil {
		return err
	}
	var link RequestDocument
	if err := r.db.First(&link, "access_request_id = ? AND document_id = ?", appID, documentID).Error; err != nil {
		return err
	}
	line := computeLineTotal(link.AccessLevel, qty, additionalFactor(svc.Type))
	return r.db.Model(&RequestDocument{}).
		Where("access_request_id = ? AND document_id = ?", appID, documentID).
		Updates(map[string]interface{}{
			"quantity":   qty,
			"line_total": line,
			"position":   pos,
			"is_primary": primary,
		}).Error
}

func (r *Repository) DeleteRequestDocument(appID, documentID uint) error {
	return r.db.Where("access_request_id = ? AND document_id = ?", appID, documentID).
		Delete(&RequestDocument{}).Error
}

// Обновить уровень доступа услуги в заявке
func (r *Repository) UpdateRequestDocumentAccessLevel(appID, documentID uint, level float64) error {
	if level != 1 && level != 2 {
		return fmt.Errorf("unsupported access level")
	}

	var document Document
	if err := r.db.First(&document, documentID).Error; err != nil {
		return err
	}

	extra := additionalFactor(document.Type)
	line := computeLineTotal(level, 1, extra)

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RequestDocument{}).
			Where("access_request_id = ? AND document_id = ?", appID, documentID).
			Updates(map[string]interface{}{
				"access_level": level,
				"line_total":   line,
			}).Error; err != nil {
			return err
		}

		return r.recalcTotalsTx(tx, appID)
	})
}

func computeLineTotal(level float64, qty int, extra float64) float64 {
	return float64(qty)*level + float64(qty)*extra
}

func additionalFactor(documentType string) float64 {
	// Простое правило: за "чувствительные" типы добавляем единицу к коэффициенту
	if documentType == "A" || strings.ToLower(documentType) == "sql" {
		return 1
	}
	return 0
}

func (r *Repository) GetUserByEmail(email string) (User, error) {
	var user User
	err := r.db.Where("email = ?", email).First(&user).Error
	return user, err
}

func (r *Repository) GetUserByID(id uint) (User, error) {
	var user User
	err := r.db.First(&user, id).Error
	return user, err
}
