-- 文件作用：创建用户表
-- 说明：定义系统用户表，包含账号、邮箱、密码哈希、昵称、头像、简介、状态等字段，支持软删除

-- 创建用户表，存储系统用户的基本信息、认证凭据和账号状态
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username varchar(64) NOT NULL UNIQUE,
    email varchar(255) NOT NULL UNIQUE,
    password_hash varchar(255) NOT NULL,
    nickname varchar(64),
    avatar_url text,
    bio varchar(500),
    status varchar(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

-- 创建索引，加速按状态和删除标记的筛选查询
CREATE INDEX idx_users_status_deleted ON users(status, deleted_at);
