package controllers

import (
	"airqo-integrator/models"
	"github.com/gin-gonic/gin"
	"net/http"
)

type AdminController struct{}

func (a *AdminController) ClearRequestsByBatch(c *gin.Context) {
	batchID := c.Param("batch")
	result := make(chan gin.H)
	go func() {
		models.ClearBatchRequests(batchID)
		result <- gin.H{"message": "Clearing batch requests in background"}
	}()
	c.AbortWithStatusJSON(http.StatusOK, <-result)
}

func (a *AdminController) ClearRequestsByDistrict(c *gin.Context) {
	district := c.Param("district")
	result := make(chan gin.H)
	go func() {
		models.ClearBatchRequests(district)
		result <- gin.H{"message": "Clearing district requests in background"}
	}()
	c.AbortWithStatusJSON(http.StatusOK, <-result)
}
