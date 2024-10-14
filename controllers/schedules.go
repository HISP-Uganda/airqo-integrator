package controllers

import (
	"airqo-integrator/models"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"net/http"
	"strconv"
	"time"
)

type ScheduleController struct{}

// NewSchedule creates a new Schedule
func (s *ScheduleController) NewSchedule(c *gin.Context) {
	// the JSON body of the post request should bind to the Schedule struct and thereafter create the Schedule
	db := c.MustGet("dbConn").(*sqlx.DB)
	var schedule models.Schedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error SCHED-001": err.Error()})
		return
	}
	schedule.Created = time.Now().In(models.Location)
	schedule.Updated = time.Now().In(models.Location)
	id, err := models.CreateSchedule(db, schedule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error SCHED-002": err.Error()})
		return
	}
	schedule, _ = models.GetSchedule(db, id)
	c.JSON(http.StatusCreated, schedule)

}

func (s *ScheduleController) ListSchedules(c *gin.Context) {
	db := c.MustGet("dbConn").(*sqlx.DB)
	schedules := models.ListSchedules(db)
	c.JSON(http.StatusOK, schedules)
}

func (s *ScheduleController) GetSchedule(c *gin.Context) {
	db := c.MustGet("dbConn").(*sqlx.DB)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	schedule, err := models.GetSchedule(db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// DeleteSchedule deletes a schedule given id is params
func (s *ScheduleController) DeleteSchedule(c *gin.Context) {
	db := c.MustGet("dbConn").(*sqlx.DB)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	err = models.DeleteSchedule(db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// UpdateSchedule updates a schedule given id is params

func (s *ScheduleController) UpdateSchedule(c *gin.Context) {
	db := c.MustGet("dbConn").(*sqlx.DB)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	var schedule models.Schedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	schedule.ID = id
	err = models.UpdateSchedule(db, schedule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
