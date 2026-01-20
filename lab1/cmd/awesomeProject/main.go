package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"lab1/internal/app/config"
	"lab1/internal/app/handler"
	"lab1/internal/app/repository"
	"lab1/internal/pkg"
)

func main() {
	// Создаём маршрутизатор Gin
	router := gin.Default()

	// === 1. Загружаем конфиг ===
	conf, err := config.NewConfig()
	if err != nil {
		logrus.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// === 2. Подключаемся к базе данных через GORM ===
	rep, err := repository.New()
	if err != nil {
		log.Fatalf("Ошибка инициализации репозитория (БД): %v", err)
	}

	fmt.Println("Подключение к PostgreSQL успешно")

	// === 3. Создаём обработчик (handler) ===
	hand := handler.NewHandler(rep)

	// === 4. Регистрируем API и статическую часть ===
	hand.RegisterAPI(router)
	application := pkg.NewApp(conf, router, hand)
	application.RunApp()
}
