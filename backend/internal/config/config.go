package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath 默认配置文件路径，兼容从 backend 目录或项目根目录启动
const DefaultConfigPath = "configs/config.yaml"

// Config 应用总配置，与 configs/config.yaml 一一对应
type Config struct {
	Server        Server        `yaml:"server"`
	Database      Database      `yaml:"database"`
	Redis         Redis         `yaml:"redis"`
	RabbitMQ      RabbitMQ      `yaml:"rabbitmq"`
	JWT           JWT           `yaml:"jwt"`
	Observability Observability `yaml:"observability"`
}

// Server HTTP 服务配置
type Server struct {
	Port int `yaml:"port"`
}

// Database MySQL 数据库配置
type Database struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// Redis 配置
type Redis struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// RabbitMQ 消息队列配置
type RabbitMQ struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// JWT 鉴权配置
type JWT struct {
	Secret             string `yaml:"secret"`
	ExpireHours        int    `yaml:"expire_hours"`
	RefreshExpireHours int    `yaml:"refresh_expire_hours"`
}

// Observability 可观测性配置
type Observability struct {
	Pprof Pprof `yaml:"pprof"`
}

// Pprof pprof 性能分析配置
type Pprof struct {
	Enabled    bool   `yaml:"enabled"`
	APIAddr    string `yaml:"api_addr"`
	WorkerAddr string `yaml:"worker_addr"`
}

// DefaultConfig 返回带默认值的配置，文件中缺失的字段会用默认值兜底
func DefaultConfig() *Config {
	return &Config{
		Server: Server{Port: 9000},
		Database: Database{
			Host:   "localhost",
			Port:   3306,
			User:   "root",
			DBName: "feed",
		},
		Redis: Redis{
			Host: "localhost",
			Port: 6379,
			DB:   0,
		},
		RabbitMQ: RabbitMQ{
			Host: "localhost",
			Port: 5672,
		},
		JWT: JWT{
			Secret:             "feed-secret-change-me",
			ExpireHours:        24,
			RefreshExpireHours: 168,
		},
		Observability: Observability{
			Pprof: Pprof{
				APIAddr:    "localhost:6060",
				WorkerAddr: "localhost:6061",
			},
		},
	}
}

// Load 从指定路径加载配置文件，先填充默认值，再被文件内容覆盖
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("配置文件路径不能为空")
	}

	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", resolved, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", resolved, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置校验失败: %w", err)
	}
	return cfg, nil
}

// LoadDefault 从默认路径加载配置文件，可用环境变量 CONFIG_PATH 覆盖路径
func LoadDefault() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = DefaultConfigPath
	}
	return Load(path)
}

// resolvePath 解析配置文件路径：直接使用给定路径；相对路径兼容从项目根目录启动的场景
func resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	candidates := []string{
		path,
		filepath.Join("backend", path),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return path, nil
}

// Validate 校验关键配置项的合法性
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法: %d", c.Server.Port)
	}
	if c.Database.Host == "" || c.Database.DBName == "" {
		return errors.New("database.host / database.dbname 不能为空")
	}
	if c.Redis.Host == "" {
		return errors.New("redis.host 不能为空")
	}
	if c.RabbitMQ.Host == "" {
		return errors.New("rabbitmq.host 不能为空")
	}
	if c.JWT.Secret == "" {
		return errors.New("jwt.secret 不能为空")
	}
	if c.JWT.ExpireHours <= 0 {
		return errors.New("jwt.expire_hours 必须大于 0")
	}
	if c.JWT.RefreshExpireHours <= 0 {
		return errors.New("jwt.refresh_expire_hours 必须大于 0")
	}
	return nil
}
