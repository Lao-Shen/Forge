// Package auth provides user registration, login and password management.
// It is storage-agnostic: queries run through a table name on *gorm.DB,
// so it works with any user table that has the standard columns
// (id, username, password, avatar, created_at).
package auth

import (
	"errors"
	"fmt"
	"time"

	"earwind.top/forge/jwt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User 认证包使用的用户结构
//
// 只包含认证相关字段，项目自己的模型可以有更多字段，
// 只要表里有这些列即可正常工作。
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Avatar    string    `gorm:"size:500" json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
}

// Service 认证服务
//
//	service := auth.New(db, jwtInstance, "users")
//	token, err := service.Login("foo", "secret123")
type Service struct {
	db    *gorm.DB
	table string
	jwt   *jwt.JWT
}

// New 创建认证服务
//
//	db    - GORM 实例（自动建表由调用方负责）
//	j     - forge/jwt 实例
//	table - 用户表名
func New(db *gorm.DB, j *jwt.JWT, table string) *Service {
	return &Service{db: db, table: table, jwt: j}
}

// Register 注册新用户
//
//	规则：用户名 3~50 字符，密码至少 6 字符，用户名唯一。
//	密码用 bcrypt 加密后入库，原始密码不会存储。
func (s *Service) Register(username, password string) (*User, error) {
	if len(username) < 3 || len(username) > 50 {
		return nil, errors.New("用户名长度须在 3~50 个字符之间")
	}
	if len(password) < 6 {
		return nil, errors.New("密码长度不能少于 6 个字符")
	}

	var count int64
	if err := s.db.Table(s.table).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	u := &User{Username: username, Password: string(hashed)}
	if err := s.db.Table(s.table).Create(u).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return u, nil
}

// Login 登录：验证密码 → 签发 JWT token
func (s *Service) Login(username, password string) (string, error) {
	u, err := s.findByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("用户名或密码错误")
		}
		return "", fmt.Errorf("查询用户失败: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return "", errors.New("用户名或密码错误")
	}

	token, err := s.jwt.Generate(u.ID, u.Username)
	if err != nil {
		return "", fmt.Errorf("生成 token 失败: %w", err)
	}

	return token, nil
}

// GetUser 获取用户信息（不含密码）
func (s *Service) GetUser(userID uint) (*User, error) {
	var u User
	err := s.db.Table(s.table).First(&u, userID).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ChangePassword 修改密码：验证旧密码 → 加密新密码 → 保存
func (s *Service) ChangePassword(userID uint, oldPwd, newPwd string) error {
	u, err := s.GetUser(userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPwd)); err != nil {
		return errors.New("旧密码错误")
	}

	if len(newPwd) < 6 {
		return errors.New("新密码长度不能少于 6 个字符")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	return s.db.Table(s.table).Model(&User{}).Where("id = ?", userID).
		Update("password", string(hashed)).Error
}

// UpdateAvatar 更新用户头像路径
func (s *Service) UpdateAvatar(userID uint, avatar string) error {
	return s.db.Table(s.table).Model(&User{}).Where("id = ?", userID).
		Update("avatar", avatar).Error
}

// findByUsername 内部查询
func (s *Service) findByUsername(username string) (*User, error) {
	var u User
	err := s.db.Table(s.table).Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}
