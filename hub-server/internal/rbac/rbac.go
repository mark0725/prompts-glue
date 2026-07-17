// Package rbac 提供基于角色的访问控制：角色、权限、角色成员与权限校验。
//
// 管理端（manager 域）用户通过角色获得权限。引导管理员默认拥有 admin 角色（含全部权限）。
// 管理端写操作通过 HasPermission 校验当前用户是否具备所需权限代码。
package rbac

import (
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"hub-server/internal/model"
)

const RoleCodeAdmin = "admin"

// permissionSeed 内置权限定义（按 code 前缀分组）。
var permissionSeed = []struct {
	code, name, group string
}{
	{"user:read", "查看用户", "user"},
	{"user:write", "管理用户", "user"},
	{"user:token", "管理用户令牌", "user"},
	{"role:read", "查看角色", "role"},
	{"role:write", "管理角色", "role"},
	{"dict:read", "查看字典", "dict"},
	{"dict:write", "管理字典", "dict"},
	{"config:read", "查看配置", "config"},
	{"config:write", "管理配置", "config"},
	{"risk:read", "查看风险", "risk"},
	{"risk:write", "管理风险", "risk"},
	{"audit:read", "查看审计", "audit"},
	{"cache:read", "查看缓存", "cache"},
	{"cache:write", "管理缓存", "cache"},
	{"ldap:sync", "LDAP 同步", "ldap"},
	{"generation:read", "查看生成记录", "generation"},
}

type Service struct {
	db  *gorm.DB
	log *slog.Logger
}

func NewService(db *gorm.DB, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{db: db, log: log}
}

// SeedDefaults 初始化内置权限与 admin 角色（拥有全部权限，不可删除）。
func (s *Service) SeedDefaults() error {
	if err := s.seedPermissions(); err != nil {
		return err
	}
	return s.seedAdminRole()
}

func (s *Service) seedPermissions() error {
	for i, p := range permissionSeed {
		perm := model.Permission{Code: p.code, Name: p.name, Group: p.group, SortOrder: i + 1}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "group", "sort_order"}),
		}).Create(&perm).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) seedAdminRole() error {
	var role model.Role
	err := s.db.Where("code = ?", RoleCodeAdmin).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = model.Role{Code: RoleCodeAdmin, Name: "管理员", Description: "拥有全部管理权限", Enabled: true, System: true}
		if err := s.db.Create(&role).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		// 迁移后 is_system 列可能为默认 false，确保内置 admin 标记为系统角色。
		if !role.System {
			if err := s.db.Model(&role).Where("id = ?", role.ID).Update("is_system", true).Error; err != nil {
				return err
			}
		}
	}
	// admin 角色绑定全部权限。
	var perms []model.Permission
	if err := s.db.Find(&perms).Error; err != nil {
		return err
	}
	for _, p := range perms {
		rp := model.RolePermission{RoleID: role.ID, PermissionID: p.ID}
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rp).Error; err != nil {
			return err
		}
	}
	return nil
}

// HasAnyRole 返回用户是否为任意已启用角色的成员（管理端登录门禁：无任何角色的用户不能登录管理端）。
func (s *Service) HasAnyRole(userID string) bool {
	var count int64
	err := s.db.Table("role_members").
		Joins("JOIN roles ON roles.id = role_members.role_id").
		Where("role_members.user_id = ? AND roles.enabled = ?", userID, true).
		Count(&count).Error
	return err == nil && count > 0
}

// AssignRole 按角色 code 为用户建立角色成员关系（已存在则跳过）。
func (s *Service) AssignRole(userID, roleCode string) error {
	var role model.Role
	if err := s.db.Where("code = ?", roleCode).First(&role).Error; err != nil {
		return err
	}
	member := model.RoleMember{RoleID: role.ID, UserID: userID}
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&member).Error; err != nil {
		return err
	}
	return nil
}

// HasPermission 返回 manager 用户是否拥有指定权限代码（其所有角色权限的并集）。
func (s *Service) HasPermission(userID, code string) bool {
	var count int64
	err := s.db.Table("role_members").
		Joins("JOIN role_permissions ON role_permissions.role_id = role_members.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Joins("JOIN roles ON roles.id = role_members.role_id").
		Where("role_members.user_id = ? AND permissions.code = ? AND roles.enabled = ?", userID, code, true).
		Count(&count).Error
	return err == nil && count > 0
}

// ---- 角色管理 ----

type RoleDetail struct {
	model.Role
	PermissionIDs []string `json:"permission_ids"`
}

func (s *Service) ListRoles() ([]model.Role, error) {
	var roles []model.Role
	if err := s.db.Order("is_system desc, created_at asc").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (s *Service) GetRole(id string) (RoleDetail, error) {
	var role model.Role
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return RoleDetail{}, err
	}
	var ids []string
	if err := s.db.Model(&model.RolePermission{}).Where("role_id = ?", id).Pluck("permission_id", &ids).Error; err != nil {
		return RoleDetail{}, err
	}
	return RoleDetail{Role: role, PermissionIDs: ids}, nil
}

func (s *Service) CreateRole(code, name, description string, enabled bool) (model.Role, error) {
	if code == "" {
		return model.Role{}, errors.New("role code is required")
	}
	role := model.Role{Code: code, Name: name, Description: description, Enabled: enabled}
	if err := s.db.Create(&role).Error; err != nil {
		return model.Role{}, err
	}
	return role, nil
}

func (s *Service) UpdateRole(id, name, description string, enabled *bool) error {
	updates := map[string]any{}
	if name != "" {
		updates["name"] = name
	}
	if description != "" {
		updates["description"] = description
	}
	if enabled != nil {
		updates["enabled"] = *enabled
	}
	if len(updates) == 0 {
		return nil
	}
	return s.db.Model(&model.Role{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteRole 删除非系统角色并清理关联。
func (s *Service) DeleteRole(id string) error {
	var role model.Role
	if err := s.db.First(&role, "id = ?", id).Error; err != nil {
		return err
	}
	if role.System {
		return errors.New("system role cannot be deleted")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&model.RoleMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&role).Error
	})
}

// SetRolePermissions 全量覆盖角色的权限集合。
func (s *Service) SetRolePermissions(roleID string, permissionIDs []string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		for _, pid := range permissionIDs {
			if err := tx.Create(&model.RolePermission{RoleID: roleID, PermissionID: pid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListPermissions 返回全部权限（已启用排序）。
func (s *Service) ListPermissions() ([]model.Permission, error) {
	var perms []model.Permission
	if err := s.db.Order("`group` asc, sort_order asc, code asc").Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

// RoleMemberInfo 角色成员摘要。
type RoleMemberInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func (s *Service) ListRoleMembers(roleID string) ([]RoleMemberInfo, error) {
	var members []RoleMemberInfo
	err := s.db.Table("role_members").
		Select("manager_users.id as id, manager_users.username as username, manager_users.display_name as name").
		Joins("JOIN manager_users ON manager_users.id = role_members.user_id").
		Where("role_members.role_id = ? AND manager_users.deleted_at IS NULL", roleID).
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

// SetRoleMembers 全量覆盖角色成员（manager 用户）。
func (s *Service) SetRoleMembers(roleID string, userIDs []string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMember{}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, uid := range userIDs {
			if err := tx.Create(&model.RoleMember{RoleID: roleID, UserID: uid, CreatedAt: now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListManagerUsers 列出可选为成员的 manager 用户（供成员选择器）。
func (s *Service) ListManagerUsers() ([]RoleMemberInfo, error) {
	var users []RoleMemberInfo
	err := s.db.Table("manager_users").
		Select("id, username, display_name as name").
		Where("deleted_at IS NULL").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}
