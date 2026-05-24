package pkg

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"lab1/internal/app/config"
	"lab1/internal/app/handler"
)

type Server struct {
	Config  *config.Config
	Router  *gin.Engine
	Handler *handler.Handler
}

func NewApp(c *config.Config, r *gin.Engine, h *handler.Handler) *Server {
	return &Server{
		Config:  c,
		Router:  r,
		Handler: h,
	}
}

func (a *Server) RunApp() {
	logrus.Info("Server start up")

	a.Handler.RegisterHandler(a.Router)
	a.Handler.RegisterStatic(a.Router)

	serverAddress := fmt.Sprintf("%s:%d", a.Config.ServiceHost, a.Config.ServicePort)
	if err := a.Router.Run(serverAddress); err != nil {
		logrus.Fatal(err)
	}
	logrus.Info("Server down")
}
