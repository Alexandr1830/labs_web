package api

import (
	"log"
	"lab1/internal/app/handler"
	"lab1/internal/app/repository"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"html/template"
)


func StartServer() {
	log.Println("Starting server")

	repo, err := repository.NewRepository()
	if err != nil {
		logrus.Error("ошибка инициализации репозитория")
	}

	handler := handler.NewHandler(repo)

	r := gin.Default()
	r.SetFuncMap(template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	})
	// добавляем наш html/шаблон
	r.LoadHTMLGlob("../../templates/*")
	r.Static("/resources", "../../resources")

	r.GET("/hello", handler.GetOrders)
	r.GET("/order/:id", handler.GetOrder)
	r.GET("/edit/:id", handler.GetApplication)


	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
	log.Println("Server down")
}
