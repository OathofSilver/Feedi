# 项目技术栈与开发约束

本文件定义 feed 项目的技术选型与编码规范，AI 生成的代码必须遵守，禁止突破以下边界。

## 技术栈

| 类别 | 选型 | 禁止项 |
|------|------|--------|
| 语言 | Go 1.23.1 | — |
| Web 框架 | gin | echo、fiber |
| 配置读取 | gopkg.in/yaml.v3 | viper、envconfig |
| 数据库 | MySQL + GORM | 裸 go-sql-driver 手写 SQL（复杂查询除外） |
| 缓存 | Redis（go-redis/v9） | — |
| 队列 | RabbitMQ（rabbitmq/amqp091-go） | — |
| 日志 | 标准库 log/slog | zap、logrus 等第三方日志库 |
| 依赖管理 | go mod | 不引入未在 go.mod 声明的依赖 |






