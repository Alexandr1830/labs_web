package main

import (
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

	// Migrate the schema
	err = db.AutoMigrate(
    &repository.User{},
    &repository.Document{},
    &repository.Application{},
    &repository.ApplicationDocument{},
)
	if err != nil {
		panic("cant migrate db")
	}
}