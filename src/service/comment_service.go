package service

import (
	"database/sql"
	"task-manager/src/models"
)

type CommentService struct {
	db          *sql.DB
	taskService *TaskService
}

func NewCommentService(db *sql.DB, taskService *TaskService) *CommentService {
	return &CommentService{
		db:          db,
		taskService: taskService,
	}
}

func (s *CommentService) CreateComment(taskID, userID int, text string) (*models.TaskComment, error) {
	// Verify user can access the task (checks team membership)
	_, err := s.taskService.GetTaskByID(taskID, userID)
	if err != nil {
		return nil, err
	}

	query := "INSERT INTO task_comments (task_id, user_id, text) VALUES (?, ?, ?)"
	res, err := s.db.Exec(query, taskID, userID, text)
	if err != nil {
		return nil, err
	}

	commentID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var comment models.TaskComment
	err = s.db.QueryRow("SELECT id, task_id, user_id, text, created_at FROM task_comments WHERE id = ?", commentID).
		Scan(&comment.ID, &comment.TaskID, &comment.UserID, &comment.Text, &comment.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &comment, nil
}

func (s *CommentService) GetTaskComments(taskID, viewerID int) ([]models.TaskComment, error) {
	// Verify user can access the task (checks team membership)
	_, err := s.taskService.GetTaskByID(taskID, viewerID)
	if err != nil {
		return nil, err
	}

	query := "SELECT id, task_id, user_id, text, created_at FROM task_comments WHERE task_id = ? ORDER BY created_at ASC"
	rows, err := s.db.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.TaskComment
	for rows.Next() {
		var c models.TaskComment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.UserID, &c.Text, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}
