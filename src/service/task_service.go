package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"task-manager/src/models"
)

type TaskService struct {
	db          *sql.DB
	rdb         *redis.Client
	teamService *TeamService
}

func NewTaskService(db *sql.DB, rdb *redis.Client, teamService *TeamService) *TaskService {
	return &TaskService{
		db:          db,
		rdb:         rdb,
		teamService: teamService,
	}
}

// Helper to check if a user is a member of a team
func (s *TaskService) checkTeamMembership(userID, teamID int) error {
	role, err := s.teamService.GetUserRole(userID, teamID)
	if err != nil {
		return err
	}
	if role == "" {
		return errors.New("access denied: user is not a member of the team")
	}
	return nil
}

func (s *TaskService) CreateTask(teamID, creatorID int, req models.CreateTaskReq) (*models.Task, error) {
	// Verify creator is member
	if err := s.checkTeamMembership(creatorID, teamID); err != nil {
		return nil, err
	}

	// Verify assignee (if set) is member
	if req.AssigneeID != nil {
		if err := s.checkTeamMembership(*req.AssigneeID, teamID); err != nil {
			return nil, fmt.Errorf("assignee is not in the team: %w", err)
		}
	}

	query := "INSERT INTO tasks (title, description, status, team_id, assignee_id, created_by) VALUES (?, ?, 'todo', ?, ?, ?)"
	res, err := s.db.Exec(query, req.Title, req.Description, teamID, req.AssigneeID, creatorID)
	if err != nil {
		return nil, err
	}

	taskID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Invalidate team analytics cache
	s.invalidateTeamAnalyticsCache(teamID)

	return s.GetTaskByID(int(taskID), creatorID)
}

func (s *TaskService) GetTeamTasks(teamID, viewerID int, statusFilter string, assigneeFilter *int) ([]models.Task, error) {
	if err := s.checkTeamMembership(viewerID, teamID); err != nil {
		return nil, err
	}

	query := "SELECT id, title, description, status, team_id, assignee_id, created_by, created_at, updated_at FROM tasks WHERE team_id = ?"
	args := []interface{}{teamID}

	if statusFilter != "" {
		query += " AND status = ?"
		args = append(args, statusFilter)
	}

	if assigneeFilter != nil {
		query += " AND assignee_id = ?"
		args = append(args, *assigneeFilter)
	}

	query += " ORDER BY id DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.TeamID, &t.AssigneeID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *TaskService) GetTaskByID(taskID, viewerID int) (*models.Task, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("task:details:%d", taskID)

	var task models.Task
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(cached), &task); err == nil {
			// Check membership using cached task's team_id
			if err := s.checkTeamMembership(viewerID, task.TeamID); err != nil {
				return nil, err
			}
			return &task, nil
		}
	}

	// Query from DB
	query := "SELECT id, title, description, status, team_id, assignee_id, created_by, created_at, updated_at FROM tasks WHERE id = ?"
	err = s.db.QueryRow(query, taskID).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.TeamID, &task.AssigneeID, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("task not found")
		}
		return nil, err
	}

	// Check membership
	if err := s.checkTeamMembership(viewerID, task.TeamID); err != nil {
		return nil, err
	}

	// Cache task details
	taskJSON, _ := json.Marshal(task)
	_ = s.rdb.Set(ctx, cacheKey, taskJSON, 1*time.Hour).Err()

	return &task, nil
}

func (s *TaskService) UpdateTask(taskID, updaterID int, req models.UpdateTaskReq) (*models.Task, error) {
	// Start a transaction for the update and audit log inserts
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}

	// Fetch current task state inside transaction for update lock
	var current models.Task
	queryFetch := "SELECT id, title, description, status, team_id, assignee_id, created_by, created_at, updated_at FROM tasks WHERE id = ? FOR UPDATE"
	err = tx.QueryRow(queryFetch, taskID).Scan(&current.ID, &current.Title, &current.Description, &current.Status, &current.TeamID, &current.AssigneeID, &current.CreatedBy, &current.CreatedAt, &current.UpdatedAt)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("task not found")
		}
		return nil, err
	}

	// Check membership
	role, err := s.teamService.GetUserRole(updaterID, current.TeamID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if role == "" {
		tx.Rollback()
		return nil, errors.New("access denied: user is not a member of the team")
	}

	// Perform checks on input fields and build dynamic update statement
	changes := make(map[string][2]*string) // fieldName -> [oldVal, newVal]

	if req.Title != nil && *req.Title != current.Title {
		oldVal := current.Title
		changes["title"] = [2]*string{&oldVal, req.Title}
		current.Title = *req.Title
	}

	if req.Description != nil {
		oldDesc := current.Description
		newDesc := req.Description
		if oldDesc == nil && newDesc != nil {
			changes["description"] = [2]*string{nil, newDesc}
			current.Description = newDesc
		} else if oldDesc != nil && newDesc == nil {
			changes["description"] = [2]*string{oldDesc, nil}
			current.Description = nil
		} else if oldDesc != nil && newDesc != nil && *oldDesc != *newDesc {
			changes["description"] = [2]*string{oldDesc, newDesc}
			current.Description = newDesc
		}
	}

	if req.Status != nil && *req.Status != current.Status {
		// Validate status
		st := *req.Status
		if st != "todo" && st != "in_progress" && st != "review" && st != "done" {
			tx.Rollback()
			return nil, errors.New("invalid status value")
		}
		oldVal := current.Status
		changes["status"] = [2]*string{&oldVal, req.Status}
		current.Status = st
	}

	if req.AssigneeID != nil {
		// Check team membership of new assignee if setting it
		var oldAssigneeVal *string
		if current.AssigneeID != nil {
			val := strconv.Itoa(*current.AssigneeID)
			oldAssigneeVal = &val
		}

		if *req.AssigneeID == 0 {
			// Unassigning
			if current.AssigneeID != nil {
				changes["assignee_id"] = [2]*string{oldAssigneeVal, nil}
				current.AssigneeID = nil
			}
		} else {
			// Assigning to a user
			if current.AssigneeID == nil || *current.AssigneeID != *req.AssigneeID {
				if err := s.checkTeamMembership(*req.AssigneeID, current.TeamID); err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("new assignee is not in the team: %w", err)
				}
				val := strconv.Itoa(*req.AssigneeID)
				changes["assignee_id"] = [2]*string{oldAssigneeVal, &val}
				current.AssigneeID = req.AssigneeID
			}
		}
	}

	// If there are no changes, just commit and return
	if len(changes) == 0 {
		tx.Commit()
		return &current, nil
	}

	// Update task in database
	queryUpdate := "UPDATE tasks SET title = ?, description = ?, status = ?, assignee_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"
	_, err = tx.Exec(queryUpdate, current.Title, current.Description, current.Status, current.AssigneeID, taskID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Log histories
	queryHistory := "INSERT INTO task_history (task_id, changed_by, field_name, old_value, new_value) VALUES (?, ?, ?, ?, ?)"
	for fieldName, vals := range changes {
		_, err = tx.Exec(queryHistory, taskID, updaterID, fieldName, vals[0], vals[1])
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Invalidate Redis caches
	s.invalidateTaskCache(taskID)
	s.invalidateTeamAnalyticsCache(current.TeamID)

	return &current, nil
}

func (s *TaskService) GetTaskHistory(taskID, viewerID int) ([]models.TaskHistory, error) {
	// First retrieve the task to check team membership
	task, err := s.GetTaskByID(taskID, viewerID)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, task_id, changed_by, field_name, old_value, new_value, changed_at FROM task_history WHERE task_id = ? ORDER BY changed_at DESC"
	rows, err := s.db.Query(query, task.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.TaskHistory
	for rows.Next() {
		var h models.TaskHistory
		if err := rows.Scan(&h.ID, &h.TaskID, &h.ChangedBy, &h.FieldName, &h.OldValue, &h.NewValue, &h.ChangedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
}

func (s *TaskService) GetTeamAnalytics(teamID, viewerID int) (*models.TeamAnalytics, error) {
	if err := s.checkTeamMembership(viewerID, teamID); err != nil {
		return nil, err
	}

	ctx := context.Background()
	cacheKey := fmt.Sprintf("team:analytics:%d", teamID)

	// Try reading from cache
	cached, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var analytics models.TeamAnalytics
		if err := json.Unmarshal([]byte(cached), &analytics); err == nil {
			return &analytics, nil
		}
	}

	// Total tasks count
	var totalTasks int
	err = s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE team_id = ?", teamID).Scan(&totalTasks)
	if err != nil {
		return nil, err
	}

	// Status breakdown
	statusBreakdown := map[string]int{
		"todo":        0,
		"in_progress": 0,
		"review":      0,
		"done":        0,
	}
	rowsStatus, err := s.db.Query("SELECT status, COUNT(*) FROM tasks WHERE team_id = ? GROUP BY status", teamID)
	if err != nil {
		return nil, err
	}
	defer rowsStatus.Close()
	for rowsStatus.Next() {
		var status string
		var count int
		if err := rowsStatus.Scan(&status, &count); err == nil {
			statusBreakdown[status] = count
		}
	}

	// Members Productivity (Complex SQL Query with subqueries and joins)
	queryProductivity := `
		SELECT 
			u.id, 
			u.username, 
			tm.role,
			COALESCE(t_total.cnt, 0) as total_assigned,
			COALESCE(t_done.cnt, 0) as completed_tasks
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		LEFT JOIN (
			SELECT assignee_id, COUNT(*) as cnt 
			FROM tasks 
			WHERE team_id = ? 
			GROUP BY assignee_id
		) t_total ON u.id = t_total.assignee_id
		LEFT JOIN (
			SELECT assignee_id, COUNT(*) as cnt 
			FROM tasks 
			WHERE team_id = ? AND status = 'done' 
			GROUP BY assignee_id
		) t_done ON u.id = t_done.assignee_id
		WHERE tm.team_id = ?
		ORDER BY total_assigned DESC, u.username ASC`

	rowsProd, err := s.db.Query(queryProductivity, teamID, teamID, teamID)
	if err != nil {
		return nil, err
	}
	defer rowsProd.Close()

	var productivity []models.MemberProductivity
	for rowsProd.Next() {
		var mp models.MemberProductivity
		if err := rowsProd.Scan(&mp.UserID, &mp.Username, &mp.Role, &mp.TotalAssigned, &mp.CompletedTasks); err == nil {
			productivity = append(productivity, mp)
		}
	}

	// Recent activity (Complex JOIN Query)
	queryActivity := `
		SELECT 
			th.task_id,
			t.title as task_title,
			th.changed_by,
			u.username,
			th.field_name,
			th.old_value,
			th.new_value,
			th.changed_at
		FROM task_history th
		JOIN tasks t ON th.task_id = t.id
		JOIN users u ON th.changed_by = u.id
		WHERE t.team_id = ?
		ORDER BY th.changed_at DESC
		LIMIT 10`

	rowsAct, err := s.db.Query(queryActivity, teamID)
	if err != nil {
		return nil, err
	}
	defer rowsAct.Close()

	var recentActivity []models.ActivityLog
	for rowsAct.Next() {
		var al models.ActivityLog
		if err := rowsAct.Scan(&al.TaskID, &al.TaskTitle, &al.ChangedBy, &al.Username, &al.FieldName, &al.OldValue, &al.NewValue, &al.ChangedAt); err == nil {
			recentActivity = append(recentActivity, al)
		}
	}

	analytics := &models.TeamAnalytics{
		TotalTasks:      totalTasks,
		StatusBreakdown: statusBreakdown,
		Productivity:    productivity,
		RecentActivity:  recentActivity,
	}

	// Cache in Redis for 1 minute (analytics changes frequently, but a short cache helps with high load)
	analyticsJSON, err := json.Marshal(analytics)
	if err == nil {
		_ = s.rdb.Set(ctx, cacheKey, analyticsJSON, 1*time.Minute).Err()
	}

	return analytics, nil
}

func (s *TaskService) invalidateTaskCache(taskID int) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("task:details:%d", taskID)
	_ = s.rdb.Del(ctx, cacheKey).Err()
}

func (s *TaskService) invalidateTeamAnalyticsCache(teamID int) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("team:analytics:%d", teamID)
	_ = s.rdb.Del(ctx, cacheKey).Err()
}
