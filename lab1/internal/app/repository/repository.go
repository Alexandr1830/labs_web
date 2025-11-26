package repository

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	DateJoined   time.Time `gorm:"autoCreateTime"`
}

type Document struct {
	DocumentID uint      `gorm:"primaryKey;column:document_id"`
	Title      string    `gorm:"type:varchar(255);not null"`
	Type       string    `gorm:"type:varchar(10);not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`
	Status     string    `gorm:"type:varchar(50);not null"`
	ImagePath  string    `gorm:"type:varchar(255)"`

	Description string    `gorm:"type:text"`
	LastEdit    time.Time `gorm:"autoUpdateTime"`

	AuthorID   uint
	EditorID   uint
	ApproverID uint

	Author   User `gorm:"foreignKey:AuthorID"`
	Editor   User `gorm:"foreignKey:EditorID"`
	Approver User `gorm:"foreignKey:ApproverID"`
}

// Единственная заявка (корзина)
type Application struct {
	ApplicationID uint      `gorm:"primaryKey;column:application_id"`
	Title         string    `gorm:"column:title"`
	Description   string    `gorm:"column:description"`
	Status        string    `gorm:"column:status"`
	CreatedAt     time.Time `gorm:"column:created_at"`

	Documents []Document `gorm:"many2many:applications_documents;joinForeignKey:ApplicationID;joinReferences:DocumentID"`
}

func (Application) TableName() string {
	return "permission_applications"
}

type ApplicationDocument struct {
	ApplicationID uint `gorm:"primaryKey;column:application_id"`
	DocumentID    uint `gorm:"primaryKey;column:document_id"`
}

func (ApplicationDocument) TableName() string {
	return "applications_documents"
}

//
// ========== ИНИЦИАЛИЗАЦИЯ ENV ==========
//

func init() {
	_ = godotenv.Load("../../.env")

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

	err = db.AutoMigrate(&User{}, &Document{}, &Application{}, &ApplicationDocument{})
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
	var docs []Document
	result := r.db.
		Preload("Author").
		Preload("Editor").
		Preload("Approver").
		Find(&docs)

	return docs, result.Error
}

func (r *Repository) GetDocumentsByTitle(title string) ([]Document, error) {
	var docs []Document
	result := r.db.
		Where("title ILIKE ?", "%"+title+"%").
		Preload("Author").
		Preload("Editor").
		Preload("Approver").
		Find(&docs)

	return docs, result.Error
}

func (r *Repository) GetDocumentByID(id uint) (Document, error) {
	var doc Document
	result := r.db.
		Preload("Author").
		Preload("Editor").
		Preload("Approver").
		First(&doc, id)

	return doc, result.Error
}

//
// ========== МЕТОДЫ APPLICATION (ОДНА ЗАЯВКА) ==========
//

// Найти или создать единственную заявку
func (r *Repository) GetOrCreateDraftApplication() (Application, error) {
    var app Application

    result := r.db.
        Preload("Documents").
        Preload("Documents.Author").
        Preload("Documents.Editor").
        Preload("Documents.Approver").
        Where("status = ? AND is_deleted = FALSE", "draft").
        First(&app)

    if result.Error == nil {
        return app, nil
    }

    // Создаём новую заявку
    app = Application{
        Title:  "Заявка",
        Status: "draft",
    }

    if err := r.db.Create(&app).Error; err != nil {
        return Application{}, err
    }

    return app, nil
}

// Добавить документ в заявку
func (r *Repository) AddDocumentToApplication(appID, docID uint) error {
	var app Application
	var doc Document

	if err := r.db.First(&app, appID).Error; err != nil {
		return err
	}
	if err := r.db.First(&doc, docID).Error; err != nil {
		return err
	}

	return r.db.Model(&app).Association("Documents").Append(&doc)
}

// Очистить заявку (полностью убрать все документы)
func (r *Repository) ClearApplication(appID uint) error {
	var app Application

	if err := r.db.First(&app, appID).Error; err != nil {
		return err
	}

	return r.db.Model(&app).Association("Documents").Clear()
}

func (r *Repository) GetApplicationByID(id uint) (Application, error) {
	var app Application

	result := r.db.
		Preload("Documents").
		Preload("Documents.Author").
		Preload("Documents.Editor").
		Preload("Documents.Approver").
		First(&app, id)

	return app, result.Error
}

func (r *Repository) DeleteApplicationByID(id uint) error {
    return r.db.Model(&Application{}).
        Where("application_id = ?", id).
        Update("is_deleted", true).Error
}

func (r *Repository) GetUserByEmail(email string) (User, error) {
	var user User
	err := r.db.Where("email = ?", email).First(&user).Error
	return user, err
}
