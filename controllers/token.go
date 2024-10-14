package controllers

import (
	"airqo-integrator/models"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type TokenController struct{}

func (t *TokenController) GetActiveToken(c *gin.Context) {
	userID := c.MustGet("currentUser").(int64)
	user, err := models.GetUserById(userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	token, err := user.GetActiveToken()
	if err != nil {
		c.JSON(200, gin.H{
			"token": "",
		})
		return
	}
	c.JSON(200, gin.H{
		"token": token,
	})
}

func (t *TokenController) GenerateNewToken(c *gin.Context) {
	db := c.MustGet("dbConn").(*sqlx.DB)

	userID := c.MustGet("currentUser").(int64)
	_, updateErr := db.Exec("UPDATE user_apitoken SET is_active = FALSE WHERE user_id = $1", userID)
	if updateErr != nil {
		_ = c.Error(updateErr)
		return
	}

	token, err := models.GenerateToken()
	if err != nil {
		_ = c.Error(err)
		return
	}
	userToken := models.UserToken{
		UserID: userID,
		Token:  token,
	}
	userToken.Save()

	c.JSON(200, gin.H{
		"token": token,
	})
	return
}

func (t *TokenController) DeleteInactiveTokens(c *gin.Context) {
	db := c.MustGet("dbConn").(*sqlx.DB)
	userID := c.MustGet("currentUser").(int64)
	_, err := db.Exec("DELETE FROM user_apitoken WHERE is_active = FALSE AND user_id = $1", userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(200, gin.H{
		"status": "in active tokens for user deleted!",
	})
}

func (t *TokenController) RevokeToken(c *gin.Context) {

}
