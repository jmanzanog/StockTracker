package http

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine, handler *Handler) {
	router.GET("/", handler.DashboardPage)

	api := router.Group("/api/v1")
	{
		api.POST("/positions", handler.AddPosition)
		api.POST("/positions/batch", handler.AddPositionsBatch)
		api.GET("/positions", handler.ListPositions)
		api.GET("/positions/:id", handler.GetPosition)
		api.DELETE("/positions/:id", handler.DeletePosition)
		api.POST("/positions/:id/sell", handler.SellPartial)

		api.GET("/portfolio", handler.GetPortfolio)
		api.POST("/portfolio/refresh", handler.RefreshPrices)
		api.GET("/dashboard", handler.GetDashboard)
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
