.PHONY: build build-linux clean run

APP_NAME=goemail
VERSION=1.0.0

# 默认构建 (当前系统)
build:
	go build -o $(APP_NAME) main.go

# 交叉编译 Linux AMD64
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(APP_NAME)-linux-amd64 main.go

# 交叉编译 Windows AMD64
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(APP_NAME).exe main.go

# 清理
clean:
	rm -f $(APP_NAME) $(APP_NAME).exe $(APP_NAME)-linux-amd64 goemail.db

# 运行
run:
	go run main.go
