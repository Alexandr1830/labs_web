package main

import (
	"log"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"lab1/internal/app/dsn"
	"lab1/internal/app/repository"
)

func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate schema for users, documents, access_requests, m2m
	if err = db.AutoMigrate(
		&repository.User{},
		&repository.Document{},
		&repository.AccessRequest{},
		&repository.RequestDocument{},
	); err != nil {
		panic("cant migrate db")
	}

	seedUsers(db)
	seedDocuments(db)
	log.Println("migration + seed done")
}

func seedUsers(db *gorm.DB) {
	users := []repository.User{
		{Username: "Иван Иванов", Email: "ivanov@mail.ru", PasswordHash: "hash123", Role: "user", IsModerator: false},
		{Username: "Админ Админ", Email: "admin@mail.ru", PasswordHash: "admin123", Role: "admin", IsModerator: true},
		{Username: "Модератор", Email: "moderator@mail.ru", PasswordHash: "mod123", Role: "moderator", IsModerator: true},
	}

	for _, u := range users {
		db.Where(repository.User{Email: u.Email}).Assign(u).FirstOrCreate(&u)
	}
}

func seedDocuments(db *gorm.DB) {
	documents := []repository.Document{
		{Title: "Отчет по продажам Q1", Type: "xls", Status: "active", Description: "Финансовый отчет за первый квартал", AccessLevel: 1, ImageURL: "/resources/img/xls.png"}, // чтение
		{Title: "Техническая спецификация API", Type: "doc", Status: "active", Description: "Спецификация REST API", AccessLevel: 2, ImageURL: "/resources/img/doc.png"},      // запись
		{Title: "ER-диаграмма базы", Type: "sql", Status: "draft", Description: "Диаграмма текущей схемы данных", AccessLevel: 3, ImageURL: "/resources/img/sql.png"},         // чтение+запись
		{Title: "Отчет по доступам", Type: "A", Status: "active", Description: "Отчет о выдаче прав доступа", AccessLevel: 3, ImageURL: "/resources/img/A.png"},               // чтение+запись
	}

	for _, s := range documents {
		db.Where("name = ?", s.Title).Assign(s).FirstOrCreate(&s)
	}
}
