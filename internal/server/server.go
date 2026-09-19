package server

import (
	"github.com/gin-gonic/gin"

	"github.com/muammarahlnn/design-payment-system/internal/config"
)

// Server owns the gin engine and its lifecycle. Feature modules attach their
// routes via Router(); Server itself knows nothing about them.
type Server struct {
	engine *gin.Engine
	port   string
}

func New(cfg config.AppConfig) *Server {
	return &Server{
		engine: gin.Default(),
		port:   cfg.Port,
	}
}

// Router exposes the underlying engine so feature handlers can register routes.
func (s *Server) Router() gin.IRouter {
	return s.engine
}

// Run starts the HTTP server and blocks until it stops.
func (s *Server) Run() error {
	return s.engine.Run(":" + s.port)
}
