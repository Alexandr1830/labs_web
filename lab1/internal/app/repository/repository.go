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

type Service struct {
	ServiceID   uint      `gorm:"primaryKey;column:service_id"`
	Title       string    `gorm:"type:varchar(255);not null;column:name"`
	Type        string    `gorm:"type:varchar(50);not null;column:category"`
	Description string    `gorm:"type:text"`
	Status      string    `gorm:"type:varchar(50);not null;default:'active'"`
	ImageURL    string    `gorm:"type:varchar(255);column:image_url"`
	AccessLevel float64   `gorm:"type:numeric(12,2);default:1;column:price"` // 1=чтение, 2=запись, 3=чтение+запись
	IsDeleted   bool      `gorm:"column:is_deleted;default:false"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
	CreatorID   uint      `gorm:"column:creator_id"`
	Creator     User      `gorm:"foreignKey:CreatorID;references:UserID"`
}

func (Service) TableName() string { return "services" }

// Единственная заявка (корзина)
type Application struct {
	ApplicationID uint       `gorm:"primaryKey;column:application_id"`
	Title         string     `gorm:"column:title"`
	Description   string     `gorm:"column:description"`
	Status        string     `gorm:"column:status"`
	IsDeleted     bool       `gorm:"column:is_deleted;default:false"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	FormedAt      *time.Time `gorm:"column:formed_at"`
	CompletedAt   *time.Time `gorm:"column:completed_at"`
	CreatorID     uint       `gorm:"column:creator_id"`
	ModeratorID   *uint      `gorm:"column:moderator_id"`
	TotalScore    float64    `gorm:"column:total_cost;type:numeric(12,2);default:0"` // в колонке total_cost храним суммарный коэффициент доступа
	DeliveryDate  *time.Time `gorm:"column:delivery_date"`

	Services  []Service `gorm:"many2many:applications_services;joinForeignKey:ApplicationID;joinReferences:ServiceID"`
	Creator   User      `gorm:"foreignKey:CreatorID;references:UserID"`
	Moderator *User     `gorm:"foreignKey:ModeratorID;references:UserID"`

	ComputedResult int `gorm:"-"`
}

func (Application) TableName() string { return "permission_applications" }

type ApplicationService struct {
	ApplicationID uint    `gorm:"primaryKey;column:application_id"`
	ServiceID     uint    `gorm:"primaryKey;column:service_id"`
	Quantity      int     `gorm:"column:quantity;default:1"`
	Position      int     `gorm:"column:position;default:0"`
	IsPrimary     bool    `gorm:"column:is_primary;default:false"`
	LineTotal     float64 `gorm:"column:line_total;type:numeric(12,2);default:0"`
	AccessLevel   float64 `gorm:"column:access_level;type:numeric(12,2);default:1"`
}

func (ApplicationService) TableName() string {
	return "applications_services"
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

	err = db.AutoMigrate(&User{}, &Service{}, &Application{}, &ApplicationService{})
	if err != nil {
		return nil, fmt.Errorf("ошибка миграции: %v", err)
	}

	fmt.Println("Подключение к PostgreSQL успешно")
	return &Repository{db: db}, nil
}

//
// ========== МЕТОДЫ DOCUMENTS ==========
//

func (r *Repository) GetAllServices() ([]Service, error) {
	var services []Service
	result := r.db.
		Where("is_deleted = FALSE").
		Preload("Creator").
		Find(&services)

	return services, result.Error
}

func (r *Repository) GetServicesByName(name string) ([]Service, error) {
	var services []Service
	result := r.db.
		Where("is_deleted = FALSE").
		Where("name ILIKE ?", "%"+name+"%").
		Preload("Creator").
		Find(&services)

	return services, result.Error
}

func (r *Repository) GetServiceByID(id uint) (Service, error) {
	var service Service
	result := r.db.
		Where("is_deleted = FALSE").
		Preload("Creator").
		First(&service, id)

	return service, result.Error
}

// Фильтр услуг по имени и статусу
func (r *Repository) GetServicesFiltered(name, status string) ([]Service, error) {
	var services []Service
	q := r.db.Where("is_deleted = FALSE")
	if name != "" {
		q = q.Where("name ILIKE ?", "%"+name+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Preload("Creator").Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func (r *Repository) CreateService(s Service) (Service, error) {
	if s.Status == "" {
		s.Status = "active"
	}
	if err := r.db.Create(&s).Error; err != nil {
		return Service{}, err
	}
	return s, nil
}

func (r *Repository) UpdateService(id uint, title, typ, desc, status string) error {
	fields := map[string]interface{}{
		"name":        title,
		"category":    typ,
		"description": desc,
	}
	if status != "" {
		fields["status"] = status
	}
	return r.db.Model(&Service{}).
		Where("service_id = ? AND is_deleted = FALSE", id).
		Updates(fields).Error
}

func (r *Repository) DeleteService(id uint) error {
	return r.db.Model(&Service{}).
		Where("service_id = ?", id).
		Update("is_deleted", true).Error
}

// Обновить ссылку на изображение услуги, удаляя старый путь при необходимости.
func (r *Repository) UpdateServiceImage(id uint, imageURL string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var svc Service
		if err := tx.First(&svc, id).Error; err != nil {
			return err
		}
		// Физическое удаление файла, если путь локальный и отличается
		if svc.ImageURL != "" && svc.ImageURL != imageURL && strings.HasPrefix(svc.ImageURL, "uploads/") {
			_ = os.Remove(svc.ImageURL)
		}
		return tx.Model(&Service{}).Where("service_id = ?", id).Update("image_url", imageURL).Error
	})
}

//
// ========== МЕТОДЫ APPLICATION (ОДНА ЗАЯВКА) ==========
//

// Найти или создать черновик конкретного пользователя
func (r *Repository) GetOrCreateDraftApplication(userID uint) (Application, error) {
	var app Application

	result := r.db.
		Preload("Services").
		Where("status = ? AND is_deleted = FALSE AND creator_id = ?", StatusDraft, userID).
		First(&app)

	if result.Error == nil {
		return app, nil
	}

	app = Application{
		Title:     "Заявка",
		Status:    StatusDraft,
		CreatorID: userID,
	}

	if err := r.db.Create(&app).Error; err != nil {
		return Application{}, err
	}

	return app, nil
}

func (r *Repository) GetDraftApplication(userID uint) (Application, error) {
	var app Application
	err := r.db.
		Preload("Services").
		Where("status = ? AND is_deleted = FALSE AND creator_id = ?", StatusDraft, userID).
		First(&app).Error
	return app, err
}

// Добавить услугу в заявку с расчётом суммы
func (r *Repository) AddServiceToApplication(appID, serviceID uint) error {
	var app Application
	if err := r.db.Where("is_deleted = FALSE").First(&app, appID).Error; err != nil {
		return err
	}

	var service Service
	if err := r.db.Where("is_deleted = FALSE").First(&service, serviceID).Error; err != nil {
		return err
	}

	var link ApplicationService
	err := r.db.First(&link, "application_id = ? AND service_id = ?", appID, serviceID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		link = ApplicationService{
			ApplicationID: appID,
			ServiceID:     serviceID,
			Quantity:      1,
			AccessLevel:   1,
			LineTotal:     computeLineTotal(1, 1, additionalFactor(service.Type)),
		}
		if createErr := r.db.Create(&link).Error; createErr != nil {
			return createErr
		}
	} else if err == nil {
		link.Quantity++
		link.LineTotal = computeLineTotal(link.AccessLevel, link.Quantity, additionalFactor(service.Type))
		if updErr := r.db.Model(&ApplicationService{}).
			Where("application_id = ? AND service_id = ?", appID, serviceID).
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
	if err := db.Model(&ApplicationService{}).
		Select("COALESCE(SUM(line_total),0)").
		Where("application_id = ?", appID).
		Scan(&total).Error; err != nil {
		return err
	}

	return db.Model(&Application{}).
		Where("application_id = ?", appID).
		Update("total_cost", total).Error
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
func (r *Repository) ClearApplication(appID uint) error {
	var app Application

	if err := r.db.First(&app, appID).Error; err != nil {
		return err
	}

	return r.db.Model(&app).Association("Services").Clear()
}

func (r *Repository) GetApplicationByID(id uint) (Application, error) {
	var app Application

	result := r.db.
		Preload("Services").
		Preload("Creator").
		Preload("Moderator").
		Where("is_deleted = FALSE").
		First(&app, id)

	if result.Error != nil {
		return app, result.Error
	}

	// Подтягиваем уровни доступа из m-m
	var links []ApplicationService
	if err := r.db.Where("application_id = ?", id).Find(&links).Error; err != nil {
		return app, err
	}
	linkMap := make(map[uint]ApplicationService)
	for _, l := range links {
		linkMap[l.ServiceID] = l
	}
	for i := range app.Services {
		if l, ok := linkMap[app.Services[i].ServiceID]; ok {
			app.Services[i].AccessLevel = l.AccessLevel
		}
	}

	return app, nil
}

func (r *Repository) DeleteApplicationByID(id uint) error {
	// Логическое удаление через SQL UPDATE (без ORM)
	return r.db.Exec(
		"UPDATE permission_applications SET status = ?, is_deleted = TRUE WHERE application_id = ?",
		StatusDeleted, id,
	).Error
}

// Обновить поля заявки
func (r *Repository) UpdateApplicationFields(id uint, title, desc string) error {
	fields := map[string]interface{}{
		"title":       title,
		"description": desc,
	}
	// Системные поля не трогаем
	return r.db.Model(&Application{}).
		Where("application_id = ? AND is_deleted = FALSE AND status IN ?", id, []string{StatusDraft, StatusFormed}).
		Updates(fields).Error
}

// Список заявок (без удалённых/черновиков) с фильтрами по статусу и дате формирования.
func (r *Repository) ListApplicationsFiltered(status string, from, to *time.Time) ([]Application, error) {
	var apps []Application
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
		Preload("Services").
		Preload("Creator").
		Preload("Moderator").
		Find(&apps).Error
	if err != nil {
		return nil, err
	}

	// вычисляем количество услуг с ненулевым line_total как computed
	for i := range apps {
		var cnt int64
		if err := r.db.Model(&ApplicationService{}).
			Where("application_id = ? AND line_total > 0", apps[i].ApplicationID).
			Count(&cnt).Error; err == nil {
			apps[i].ComputedResult = int(cnt)
		}
	}
	return apps, nil
}

func (r *Repository) MarkApplicationSubmitted(id uint, creatorID uint) error {
	// Только создатель может формировать черновик.
	return r.db.Model(&Application{}).
		Where("application_id = ? AND creator_id = ? AND status = ?", id, creatorID, StatusDraft).
		Updates(map[string]interface{}{
			"status":    StatusFormed,
			"formed_at": time.Now(),
		}).Error
}

func (r *Repository) MarkApplicationCompleted(id uint, moderatorID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Application{}).
			Where("application_id = ? AND status = ?", id, StatusFormed).
			Updates(map[string]interface{}{
				"status":       StatusCompleted,
				"completed_at": time.Now(),
				"moderator_id": moderatorID,
				"delivery_date": time.Now().AddDate(0, 1, 0),
			}).Error; err != nil {
			return err
		}
		return r.recalcTotalsTx(tx, id)
	})
}

func (r *Repository) MarkApplicationRejected(id uint, moderatorID uint) error {
	return r.db.Model(&Application{}).
		Where("application_id = ? AND status = ?", id, StatusFormed).
		Updates(map[string]interface{}{
			"status":       StatusRejected,
			"completed_at": time.Now(),
			"moderator_id": moderatorID,
		}).Error
}

func (r *Repository) UpdateApplicationService(appID, serviceID uint, qty int) error {
	if qty <= 0 {
		qty = 1
	}
	var svc Service
	if err := r.db.First(&svc, serviceID).Error; err != nil {
		return err
	}
	var link ApplicationService
	if err := r.db.First(&link, "application_id = ? AND service_id = ?", appID, serviceID).Error; err != nil {
		return err
	}
	line := computeLineTotal(link.AccessLevel, qty, additionalFactor(svc.Type))
	return r.db.Model(&ApplicationService{}).
		Where("application_id = ? AND service_id = ?", appID, serviceID).
		Updates(map[string]interface{}{
			"quantity":   qty,
			"line_total": line,
			"position":   link.Position,
			"is_primary": link.IsPrimary,
		}).Error
}

func (r *Repository) UpdateApplicationServiceWithMeta(appID, serviceID uint, qty int, pos int, primary bool) error {
	if qty <= 0 {
		qty = 1
	}
	var svc Service
	if err := r.db.First(&svc, serviceID).Error; err != nil {
		return err
	}
	var link ApplicationService
	if err := r.db.First(&link, "application_id = ? AND service_id = ?", appID, serviceID).Error; err != nil {
		return err
	}
	line := computeLineTotal(link.AccessLevel, qty, additionalFactor(svc.Type))
	return r.db.Model(&ApplicationService{}).
		Where("application_id = ? AND service_id = ?", appID, serviceID).
		Updates(map[string]interface{}{
			"quantity":   qty,
			"line_total": line,
			"position":   pos,
			"is_primary": primary,
		}).Error
}

func (r *Repository) DeleteApplicationService(appID, serviceID uint) error {
	return r.db.Where("application_id = ? AND service_id = ?", appID, serviceID).
		Delete(&ApplicationService{}).Error
}

// Обновить уровень доступа услуги в заявке
func (r *Repository) UpdateServiceAccessLevel(appID, serviceID uint, level float64) error {
	if level != 1 && level != 2 {
		return fmt.Errorf("unsupported access level")
	}

	var service Service
	if err := r.db.First(&service, serviceID).Error; err != nil {
		return err
	}

	extra := additionalFactor(service.Type)
	line := computeLineTotal(level, 1, extra)

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ApplicationService{}).
			Where("application_id = ? AND service_id = ?", appID, serviceID).
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

func additionalFactor(serviceType string) float64 {
	// Простое правило: за "чувствительные" типы добавляем единицу к коэффициенту
	if serviceType == "A" || strings.ToLower(serviceType) == "sql" {
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
