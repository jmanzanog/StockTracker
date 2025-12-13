package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, handler *Handler) {
	api := router.Group("/api/v1")
	{
		api.POST("/positions", handler.AddPosition)
		api.GET("/positions", handler.ListPositions)
		api.GET("/positions/:id", handler.GetPosition)
		api.DELETE("/positions/:id", handler.DeletePosition)

		api.GET("/portfolio", handler.GetPortfolio)
		api.POST("/portfolio/refresh", handler.RefreshPrices)
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
