# 安装指南

本指南将帮助您在不同环境中安装和部署 AxonHub。

## 📋 系统要求

### 最低要求
- **CPU**: 2 核心
- **内存**: 4GB RAM
- **存储**: 10GB 可用空间
- **网络**: 稳定的互联网连接

### 推荐配置
- **CPU**: 4 核心
- **内存**: 8GB RAM
- **存储**: 50GB SSD
- **网络**: 100Mbps+ 带宽

### 软件依赖
- **Go**: 1.24+ ([安装指南](https://golang.org/dl/))
- **Node.js**: 18+ ([安装指南](https://nodejs.org/))
- **pnpm**: 最新版本 (`npm install -g pnpm`)
- **Git**: 用于克隆代码仓库

## 🚀 安装方式

### 方式一：二进制文件安装（推荐）

#### 下载预编译二进制文件

1. 访问 [Release 页面](https://github.com/looplj/axonhub/releases)
2. 下载适合您系统的二进制文件：
   ```bash
   # Linux AMD64
   wget https://github.com/looplj/axonhub/releases/latest/download/axonhub-linux-amd64
   
   # macOS AMD64
   wget https://github.com/looplj/axonhub/releases/latest/download/axonhub-darwin-amd64
   
   # macOS ARM64
   wget https://github.com/looplj/axonhub/releases/latest/download/axonhub-darwin-arm64
   ```

3. 设置执行权限：
   ```bash
   chmod +x axonhub-*
   ```

4. 移动到系统路径：
   ```bash
   sudo mv axonhub-* /usr/local/bin/axonhub
   ```

#### 验证安装

```bash
axonhub --version
```

### 方式二：源码编译安装

#### 克隆项目

```bash
git clone https://github.com/looplj/axonhub.git
cd axonhub
```

#### 编译项目

```bash
# 编译后端
go build -o axonhub cmd/axonhub/main.go

# 编译前端（可选）
cd frontend
pnpm install
pnpm build
cd ..
```

#### 安装二进制文件

```bash
sudo mv axonhub /usr/local/bin/
```

### 方式三：Docker 安装

#### 使用 Docker 镜像

```bash
# 拉取最新镜像
docker pull looplj/axonhub:latest

# 运行容器
docker run -d \
  --name axonhub \
  -p 8090:8090 \
  -v $(pwd)/config.yml:/root/config.yml \
  looplj/axonhub:latest
```

#### 使用 Docker Compose

```bash
# 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 复制配置文件
cp config.example.yml config.yml

# 启动服务
docker-compose up -d
```

## ⚙️ 基础配置

### 创建配置文件

```bash
# 复制示例配置文件
cp config.example.yml config.yml

# 编辑配置文件
nano config.yml
```

### 最小配置示例

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db"

log:
  level: "info"
  encoding: "json"
```

### 环境变量配置

```bash
# 创建环境变量文件
cat > .env << EOF
# 数据库配置
AXONHUB_DB_DIALECT=sqlite3
AXONHUB_DB_DSN=file:axonhub.db

# 服务器配置
AXONHUB_SERVER_PORT=8090
AXONHUB_SERVER_NAME=AxonHub

# 日志配置
AXONHUB_LOG_LEVEL=info
AXONHUB_LOG_ENCODING=json
EOF
```

## 🚀 启动服务

### 系统服务方式（推荐）

#### 创建系统用户

```bash
sudo useradd -r -s /bin/false axonhub
sudo usermod -aG axonhub $USER
```

#### 创建服务文件

创建 `/etc/systemd/system/axonhub.service`：

```ini
[Unit]
Description=AxonHub AI Gateway
After=network.target

[Service]
Type=simple
User=axonhub
Group=axonhub
WorkingDirectory=/opt/axonhub
ExecStart=/usr/local/bin/axonhub
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# 环境变量
Environment=AXONHUB_LOG_LEVEL=info
Environment=AXONHUB_SERVER_PORT=8090

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/opt/axonhub

[Install]
WantedBy=multi-user.target
```

#### 启动服务

```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start axonhub

# 设置开机自启
sudo systemctl enable axonhub

# 检查服务状态
sudo systemctl status axonhub
```

### 直接运行方式

```bash
# 前台运行
axonhub

# 后台运行
nohup axonhub > axonhub.log 2>&1 &

# 使用 tmux/screen
tmux new-session -d -s axonhub 'axonhub'
```

## 🔍 验证安装

### 检查服务状态

```bash
# 检查进程
ps aux | grep axonhub

# 检查端口
netstat -tulpn | grep 8090

# 检查日志
journalctl -u axonhub -f
```

### 测试 API 连接

```bash
# 健康检查
curl http://localhost:8090/health

# 预期响应
{"status":"ok","timestamp":"2024-01-01T00:00:00Z"}
```

### 访问管理界面

打开浏览器访问：`http://localhost:8090`

## 📊 常见问题

### 端口冲突

**问题**：`Error: listen tcp :8090: bind: address already in use`

**解决方案**：
```bash
# 查找占用端口的进程
sudo lsof -i :8090
sudo netstat -tulpn | grep :8090

# 终止占用进程
sudo kill -9 <PID>

# 或修改配置文件使用其他端口
echo "AXONHUB_SERVER_PORT=8091" >> .env
```

### 权限问题

**问题**：`Error: permission denied`

**解决方案**：
```bash
# 检查文件权限
ls -la /usr/local/bin/axonhub
ls -la /opt/axonhub

# 修复权限
sudo chown axonhub:axonhub /usr/local/bin/axonhub
sudo chown -R axonhub:axonhub /opt/axonhub
```

### 数据库连接失败

**问题**：`Error: failed to connect to database`

**解决方案**：
```bash
# 检查数据库服务状态
sudo systemctl status postgresql
sudo systemctl status mysql

# 测试数据库连接
psql -h localhost -U axonhub -d axonhub
mysql -h localhost -u axonhub -p axonhub

# 检查数据库配置
cat config.yml | grep -A5 db:
```

### 依赖缺失

**问题**：`Error: cannot find shared library`

**解决方案**：
```bash
# 安装必要的系统依赖
# Ubuntu/Debian
sudo apt update
sudo apt install -y ca-certificates tzdata

# CentOS/RHEL
sudo yum update
sudo yum install -y ca-certificates tzdata

# 重新安装二进制文件
sudo rm /usr/local/bin/axonhub
sudo cp axonhub-linux-amd64 /usr/local/bin/axonhub
sudo chmod +x /usr/local/bin/axonhub
```

## 🔄 升级指南

### 二进制文件升级

```bash
# 下载新版本
wget https://github.com/looplj/axonhub/releases/latest/download/axonhub-linux-amd64

# 停止服务
sudo systemctl stop axonhub

# 备份当前版本
sudo mv /usr/local/bin/axonhub /usr/local/bin/axonhub.backup

# 安装新版本
sudo mv axonhub-linux-amd64 /usr/local/bin/axonhub
sudo chmod +x /usr/local/bin/axonhub

# 启动服务
sudo systemctl start axonhub

# 检查状态
sudo systemctl status axonhub
```

### Docker 升级

```bash
# 拉取最新镜像
docker pull looplj/axonhub:latest

# 停止并删除旧容器
docker stop axonhub
docker rm axonhub

# 启动新容器
docker run -d \
  --name axonhub \
  -p 8090:8090 \
  -v $(pwd)/config.yml:/root/config.yml \
  looplj/axonhub:latest
```

### Docker Compose 升级

```bash
# 拉取最新镜像
docker-compose pull

# 重新构建并启动
docker-compose up -d --build

# 清理旧镜像
docker image prune -f
```

---

## 📞 获取帮助

如果在安装过程中遇到问题，请：

1. 查看日志文件：`journalctl -u axonhub -f`
2. 检查配置文件：`cat config.yml`
3. 访问 [GitHub Issues](https://github.com/looplj/axonhub/issues)
4. 加入社区讨论：[社区论坛](https://community.axonhub.dev)

---

<div align="center">

**安装完成！** 🎉

下一步：[基础配置](./basic-configuration.md)

</div>