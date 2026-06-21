package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"task-manager/src/models"
	"task-manager/src/service"
)

type TeamHandler struct {
	teamService *service.TeamService
}

func NewTeamHandler(teamService *service.TeamService) *TeamHandler {
	return &TeamHandler{teamService: teamService}
}

// POST /api/v1/teams
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var input models.CreateTeamReq
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	teamID, err := h.teamService.CreateTeam(input.Name, userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "team created successfully",
		"team_id": teamID,
		"name":    input.Name,
	})
}

// GET /api/v1/teams
func (h *TeamHandler) GetTeams(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teams, err := h.teamService.GetMyTeams(userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, teams)
}

// POST /api/v1/teams/:id/invite
func (h *TeamHandler) InviteMember(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	teamIDStr := c.Param("id")
	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var input models.InviteMemberReq
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Role != "admin" && input.Role != "member" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be either admin or member"})
		return
	}

	err = h.teamService.InviteMember(teamID, userID.(int), input.UserID, input.Role)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "member invited successfully"})
}
