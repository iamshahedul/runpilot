package services

import (
	"errors"
	"fmt"
	"strings"

	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/models"
)

type PackageService struct {
	db *db.DB
}

func NewPackageService(database *db.DB) *PackageService {
	return &PackageService{db: database}
}

// ListPackages returns all hosting packages
func (s *PackageService) ListPackages() ([]models.Package, error) {
	rows, err := s.db.Query(`
		SELECT id, name, slug, memory_max, cpu_quota, tasks_max, disk_quota, max_apps, max_domains, created_at, updated_at
		FROM packages
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []models.Package
	for rows.Next() {
		var p models.Package
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Slug,
			&p.MemoryMax,
			&p.CPUQuota,
			&p.TasksMax,
			&p.DiskQuota,
			&p.MaxApps,
			&p.MaxDomains,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		packages = append(packages, p)
	}
	return packages, nil
}

// GetPackageByID retrieves a package by ID or Slug
func (s *PackageService) GetPackage(identifier string) (*models.Package, error) {
	var p models.Package
	query := `
		SELECT id, name, slug, memory_max, cpu_quota, tasks_max, disk_quota, max_apps, max_domains, created_at, updated_at
		FROM packages
		WHERE lower(slug) = lower(?) OR lower(name) = lower(?) OR CAST(id AS TEXT) = ?
		LIMIT 1
	`
	err := s.db.QueryRow(query, identifier, identifier, identifier).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.MemoryMax,
		&p.CPUQuota,
		&p.TasksMax,
		&p.DiskQuota,
		&p.MaxApps,
		&p.MaxDomains,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, errors.New("package not found")
	}
	return &p, nil
}

// CreatePackage adds a new hosting package
func (s *PackageService) CreatePackage(pkg models.Package) (*models.Package, error) {
	if pkg.Name == "" {
		return nil, errors.New("package name is required")
	}
	if pkg.Slug == "" {
		pkg.Slug = strings.ToLower(strings.ReplaceAll(pkg.Name, " ", "-"))
	}
	if pkg.MemoryMax == "" {
		pkg.MemoryMax = "512M"
	}
	if pkg.CPUQuota == "" {
		pkg.CPUQuota = "100%"
	}
	if pkg.TasksMax <= 0 {
		pkg.TasksMax = 128
	}
	if pkg.DiskQuota == "" {
		pkg.DiskQuota = "5G"
	}
	if pkg.MaxApps <= 0 {
		pkg.MaxApps = 1
	}
	if pkg.MaxDomains <= 0 {
		pkg.MaxDomains = 2
	}

	res, err := s.db.Exec(`
		INSERT INTO packages (name, slug, memory_max, cpu_quota, tasks_max, disk_quota, max_apps, max_domains)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, pkg.Name, pkg.Slug, pkg.MemoryMax, pkg.CPUQuota, pkg.TasksMax, pkg.DiskQuota, pkg.MaxApps, pkg.MaxDomains)
	if err != nil {
		return nil, fmt.Errorf("failed to create package: %w", err)
	}

	id, _ := res.LastInsertId()
	return s.GetPackage(fmt.Sprintf("%d", id))
}

// UpdatePackage updates an existing package
func (s *PackageService) UpdatePackage(id int64, pkg models.Package) error {
	_, err := s.db.Exec(`
		UPDATE packages
		SET name = ?, memory_max = ?, cpu_quota = ?, tasks_max = ?, disk_quota = ?, max_apps = ?, max_domains = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, pkg.Name, pkg.MemoryMax, pkg.CPUQuota, pkg.TasksMax, pkg.DiskQuota, pkg.MaxApps, pkg.MaxDomains, id)
	return err
}

// DeletePackage removes a package
func (s *PackageService) DeletePackage(id int64) error {
	_, err := s.db.Exec(`DELETE FROM packages WHERE id = ?`, id)
	return err
}
