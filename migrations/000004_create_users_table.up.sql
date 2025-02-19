-- 启用 citext 扩展（确保大小写不敏感的文本类型可用）
CREATE EXTENSION IF NOT EXISTS citext;

-- 创建 users 表
CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,  -- 自增主键
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),  -- 创建时间，带时区
    name text NOT NULL,  -- 用户名
    email CITEXT UNIQUE NOT NULL,  -- 用户邮箱（大小写不敏感，唯一）
    password_hash bytea NOT NULL,  -- 密码哈希
    activated bool NOT NULL,  -- 用户是否已激活
    version integer NOT NULL DEFAULT 1  -- 版本号，默认为 1
);
