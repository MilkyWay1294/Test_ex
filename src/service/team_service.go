package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TeamService struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewTeamService(db *sql.DB, rdb *redis.Client) *TeamService {
	return &TeamService{
		db:  db,
		rdb: rdb,
	}
}

type TeamWithRole struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedBy int       `json:"created_by"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

// GetUserRole returns the role of a user in a team, leveraging Redis caching
func (s *TeamService) GetUserRole(userID, teamID int) (string, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:team:role:%d:%d", userID, teamID)

	// Try reading from Redis cache
	role, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		return role, nil
	} else if !errors.Is(err, redis.Nil) {
		// Redis error, log it but don't fail, fallback to DB
		fmt.Printf("Redis error reading role: %v\n", err)
	}

	// Read from Database
	query := "SELECT role FROM team_members WHERE user_id = ? AND team_id = ?"
	err = s.db.QueryRow(query, userID, teamID).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User is not a member of the team
			_ = s.rdb.Set(ctx, cacheKey, "", 5*time.Minute).Err() // Negative cache for 5m
			return "", nil
		}
		return "", err
	}

	// Cache the role in Redis for 24 hours
	_ = s.rdb.Set(ctx, cacheKey, role, 24*time.Hour).Err()

	return role, nil
}

// InvalidateUserRoleCache clears the Redis cache for a user's role in a team
func (s *TeamService) InvalidateUserRoleCache(userID, teamID int) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:team:role:%d:%d", userID, teamID)
	_ = s.rdb.Del(ctx, cacheKey).Err()
}

func (s *TeamService) CreateTeam(name string, creatorID int) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}

	// 1. Create team
	queryTeam := "INSERT INTO teams (name, created_by) VALUES (?, ?)"
	res, err := tx.Exec(queryTeam, name, creatorID)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	teamID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	// 2. Add creator as team owner
	queryMember := "INSERT INTO team_members (user_id, team_id, role) VALUES (?, ?, 'owner')"
	_, err = tx.Exec(queryMember, creatorID, teamID)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	// Cache in Redis
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:team:role:%d:%d", creatorID, teamID)
	_ = s.rdb.Set(ctx, cacheKey, "owner", 24*time.Hour).Err()

	return teamID, nil
}

func (s *TeamService) GetMyTeams(userID int) ([]TeamWithRole, error) {
	query := `
		SELECT t.id, t.name, t.created_by, tm.role, tm.joined_at 
		FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = ?`

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []TeamWithRole
	for rows.Next() {
		var t TeamWithRole
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.Role, &t.JoinedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, nil
}

func (s *TeamService) InviteMember(teamID, inviterID, targetUserID int, role string) error {
	// Only owner or admin can invite members
	inviterRole, err := s.GetUserRole(inviterID, teamID)
	if err != nil {
		return err
	}
	if inviterRole != "owner" && inviterRole != "admin" {
		return errors.New("only team owner or admin can invite members")
	}

	// Verify target user exists
	var existsID int
	err = s.db.QueryRow("SELECT id FROM users WHERE id = ?", targetUserID).Scan(&existsID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("target user does not exist")
		}
		return err
	}

	// Insert or update member role
	query := `
		INSERT INTO team_members (user_id, team_id, role) 
		VALUES (?, ?, ?) 
		ON DUPLICATE KEY UPDATE role = ?`
	_, err = s.db.Exec(query, targetUserID, teamID, role, role)
	if err != nil {
		return err
	}

	// Invalidate the cache for the user's role in this team
	s.InvalidateUserRoleCache(targetUserID, teamID)

	return nil
}
