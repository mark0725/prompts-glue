package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	promptssdk "prompts-sdk"
)

type PackageFile struct {
	Package PackageMeta `yaml:"package" json:"package"`
}

type PackageMeta struct {
	Name         string           `yaml:"name" json:"name"`
	Version      string           `yaml:"version" json:"version"`
	Description  string           `yaml:"description" json:"description"`
	Status       string           `yaml:"status" json:"status"`
	Category     string           `yaml:"category" json:"category"`
	Tags         []string         `yaml:"tags,omitempty" json:"tags,omitempty"`
	Author       string           `yaml:"author,omitempty" json:"author,omitempty"`
	Repository   string           `yaml:"repository,omitempty" json:"repository,omitempty"`
	Homepage     string           `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	License      string           `yaml:"license,omitempty" json:"license,omitempty"`
	Type         string           `yaml:"type" json:"type"`
	Expression   string           `yaml:"expression,omitempty" json:"expression,omitempty"`
	Dependencies []DependencyMeta `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	CreatedAt    string           `yaml:"created_at" json:"created_at"`
}

type DependencyMeta struct {
	Name            string `yaml:"name" json:"name"`
	VersionRule     string `yaml:"versionRule,omitempty" json:"versionRule,omitempty"`
	ResolvedVersion string `yaml:"resolvedVersion,omitempty" json:"resolvedVersion,omitempty"`
	Optional        bool   `yaml:"optional,omitempty" json:"optional,omitempty"`
	AffectsCompute  bool   `yaml:"affectsCompute,omitempty" json:"affectsCompute,omitempty"`
}

type StoredPackage struct {
	Meta    PackageMeta `json:"package"`
	Content string      `json:"content"`
	Path    string      `json:"path"`
}

type Repository struct {
	Path string
}

var packageNameRegexp = regexp.MustCompile(`^(@[a-z0-9][a-z0-9_-]*/)?[a-z0-9][a-z0-9_-]*$`)

func NewRepository(path string) Repository {
	return Repository{Path: path}
}

func (r Repository) Ensure() error {
	return os.MkdirAll(r.Path, 0o755)
}

func (r Repository) WritePackage(meta PackageMeta, content string) (string, error) {
	if err := validatePackageMeta(meta); err != nil {
		return "", err
	}
	if err := r.Ensure(); err != nil {
		return "", err
	}
	path := r.packagePath(meta.Name, meta.Version)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	front, err := yaml.Marshal(meta)
	if err != nil {
		return "", err
	}
	body := "---\n" + string(front) + "---\n\n" + content
	return path, os.WriteFile(path, []byte(body), 0o644)
}

func (r Repository) ImportFile(path string) (StoredPackage, error) {
	pkg, err := ReadPackageFile(path)
	if err != nil {
		return StoredPackage{}, err
	}
	written, err := r.WritePackage(pkg.Meta, pkg.Content)
	if err != nil {
		return StoredPackage{}, err
	}
	pkg.Path = written
	return pkg, nil
}

func (r Repository) ReadPackage(name string, version string) (StoredPackage, error) {
	if version == "" || version == "latest" || version == "*" {
		resolved, err := r.LatestVersion(name)
		if err != nil {
			return StoredPackage{}, err
		}
		version = resolved
	}
	path := r.packagePath(name, version)
	pkg, err := ReadPackageFile(path)
	if err != nil {
		return StoredPackage{}, err
	}
	pkg.Path = path
	return pkg, nil
}

func (r Repository) RemovePackage(name string, version string) error {
	if version == "" {
		return os.RemoveAll(filepath.Join(r.Path, packageDir(name)))
	}
	return os.Remove(r.packagePath(name, version))
}

func (r Repository) LatestVersion(name string) (string, error) {
	versions, err := r.Versions(name)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("package %s not found", name)
	}
	sort.Strings(versions)
	return versions[len(versions)-1], nil
}

func (r Repository) Versions(name string) ([]string, error) {
	dir := filepath.Join(r.Path, packageDir(name))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	versions := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		versions = append(versions, strings.TrimSuffix(entry.Name(), ".md"))
	}
	return versions, nil
}

func (r Repository) ListPackages() ([]StoredPackage, error) {
	if err := r.Ensure(); err != nil {
		return nil, err
	}
	packages := make([]StoredPackage, 0)
	if err := filepath.WalkDir(r.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		pkg, err := ReadPackageFile(path)
		if err != nil {
			return err
		}
		pkg.Path = path
		packages = append(packages, pkg)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Meta.Name == packages[j].Meta.Name {
			return packages[i].Meta.Version < packages[j].Meta.Version
		}
		return packages[i].Meta.Name < packages[j].Meta.Name
	})
	return packages, nil
}

func (r Repository) Loader() promptssdk.ContentLoader {
	return promptssdk.ContentLoaderFunc(func(ctx context.Context, ref promptssdk.DependencyRef) (promptssdk.Content, error) {
		pkg, err := r.ReadPackage(ref.Name, ref.VersionRule)
		if err != nil {
			return promptssdk.Content{}, err
		}
		// 去除 frontmatter 分隔产生的首行换行，使返回内容与 `glue content` 正文输出一致。
		return promptssdk.Content{Name: pkg.Meta.Name, Version: pkg.Meta.Version, Text: strings.TrimPrefix(pkg.Content, "\n")}, nil
	})
}

func (r Repository) packagePath(name string, version string) string {
	return filepath.Join(r.Path, packageDir(name), version+".md")
}

func ReadPackageFile(path string) (StoredPackage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredPackage{}, err
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return StoredPackage{}, fmt.Errorf("package file %s missing frontmatter", path)
	}
	parts := strings.SplitN(strings.TrimPrefix(text, "---\n"), "---\n", 2)
	if len(parts) != 2 {
		return StoredPackage{}, fmt.Errorf("package file %s has invalid frontmatter", path)
	}
	meta := PackageMeta{}
	if err := yaml.Unmarshal([]byte(parts[0]), &meta); err != nil {
		return StoredPackage{}, err
	}
	if meta.Name == "" {
		front := PackageFile{}
		if err := yaml.Unmarshal([]byte(parts[0]), &front); err != nil {
			return StoredPackage{}, err
		}
		meta = front.Package
	}
	return StoredPackage{Meta: meta, Content: parts[1], Path: path}, nil
}

func dependenciesFromSDK(deps []promptssdk.DependencyRef) []DependencyMeta {
	out := make([]DependencyMeta, 0, len(deps))
	for _, dep := range deps {
		out = append(out, DependencyMeta{Name: dep.Name, VersionRule: dep.VersionRule, ResolvedVersion: dep.VersionRule, AffectsCompute: true})
	}
	return out
}

func validatePackageMeta(meta PackageMeta) error {
	if !packageNameRegexp.MatchString(meta.Name) {
		return fmt.Errorf("invalid package name %q", meta.Name)
	}
	if meta.Version == "" {
		return errors.New("package version is required")
	}
	if meta.Description == "" {
		return errors.New("package description is required")
	}
	if meta.Status == "" {
		return errors.New("package status is required")
	}
	if meta.Category == "" {
		return errors.New("package category is required")
	}
	if meta.Type != "PLAIN" && meta.Type != "COMPOSITE" {
		return errors.New("package type must be PLAIN or COMPOSITE")
	}
	if meta.Type == "COMPOSITE" && meta.Expression == "" {
		return errors.New("composite package expression is required")
	}
	return nil
}

func packageDir(name string) string {
	return strings.TrimPrefix(name, "@")
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
