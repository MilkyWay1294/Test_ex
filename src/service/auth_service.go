package service

import (
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"task-manager/src/config"
	"task-manager/src/models"
)

type AuthService struct {
	db  *sql.DB
	cfg *config.Config
}

func NewAuthService(db *sql.DB, cfg *config.Config) *AuthService {
	return &AuthService{
		db:  db,
		cfg: cfg,
	}
}

func (s *AuthService) Register(req models.RegisterReq) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := "INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)"
	_, err = s.db.Exec(query, req.Username, req.Email, string(hashedPassword))
	if err != nil {
		// Handle duplicate entry error (username or email)
		return err
	}
	return nil
}

func (s *AuthService) Login(req models.LoginReq) (string, error) {
	var user models.User
	query := "SELECT id, username, email, password_hash, created_at FROM users WHERE email = ?"
	err := s.db.QueryRow(query, req.Email).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("invalid email or password")
		}
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return "", errors.New("invalid email or password")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(s.cfg.JWTSecret))
}
