package models

import "time"

// User описывает структуру пользователя в БД
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Team описывает структуру команды
type Team struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedBy int       `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// TeamMember описывает связь многие-ко-многим между пользователями и командами + роль
type TeamMember struct {
	ID       int       `json:"id"`
	UserID   int       `json:"user_id"`
	TeamID   int       `json:"team_id"`
	Role     string    `json:"role"` // owner, admin, member
	JoinedAt time.Time `json:"joined_at"`
}

// Структуры для входящих запросов (DTO)
type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type CreateTeamReq struct {
	Name string `json:"name" binding:"required"`
}

type InviteMemberReq struct {
	UserID int    `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"` // admin, member
}

// Task описывает структуру задачи в БД
type Task struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Status      string    `json:"status"` // todo, in_progress, review, done
	TeamID      int       `json:"team_id"`
	AssigneeID  *int      `json:"assignee_id"`
	CreatedBy   int       `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskHistory описывает историю изменений
type TaskHistory struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	ChangedBy int       `json:"changed_by"`
	FieldName string    `json:"field_name"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

// TaskComment описывает комментарии
type TaskComment struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	UserID    int       `json:"user_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// DTO для задач и комментариев
type CreateTaskReq struct {
	Title       string  `json:"title" binding:"required"`
	Description *string `json:"description"`
	AssigneeID  *int    `json:"assignee_id"`
}

type UpdateTaskReq struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"` // todo, in_progress, review, done
	AssigneeID  *int    `json:"assignee_id"`
}

type CreateCommentReq struct {
	Text string `json:"text" binding:"required"`
}

type MemberProductivity struct {
	UserID         int    `json:"user_id"`
	Username       string `json:"username"`
	Role           string `json:"role"`
	TotalAssigned  int    `json:"total_assigned"`
	CompletedTasks int    `json:"completed_tasks"`
}

type ActivityLog struct {
	TaskID    int       `json:"task_id"`
	TaskTitle string    `json:"task_title"`
	ChangedBy int       `json:"changed_by"`
	Username  string    `json:"username"`
	FieldName string    `json:"field_name"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

type TeamAnalytics struct {
	TotalTasks      int                  `json:"total_tasks"`
	StatusBreakdown map[string]int       `json:"status_breakdown"`
	Productivity    []MemberProductivity `json:"productivity"`
	RecentActivity  []ActivityLog        `json:"recent_activity"`
}
