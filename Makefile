.PHONY: help install build run dev clean test

help:
	@echo "狗狗回憶影片自動剪輯 APP - 指令列表"
	@echo ""
	@echo "make install    - 安裝所有依賴（Go + Node.js）"
	@echo "make build      - 建立前端並編譯後端"
	@echo "make run        - 啟動後端伺服器"
	@echo "make dev        - 開發模式（前端熱重載）"
	@echo "make clean      - 清理建立檔案"
	@echo "make test       - 執行測試"

install:
	@echo "安裝 Go 依賴..."
	go mod download
	@echo "安裝前端依賴..."
	cd frontend && npm install
	@echo "安裝完成！"

build:
	@echo "建立前端..."
	cd frontend && npm run build
	@echo "編譯後端..."
	go build -o dog-memory-app main.go
	@echo "建立完成！"

run:
	@echo "啟動伺服器..."
	go run main.go

dev:
	@echo "開發模式："
	@echo "1. 後端會在 http://localhost:8080 啟動"
	@echo "2. 前端會在 http://localhost:3000 啟動"
	@echo ""
	@echo "請在另一個終端機執行: cd frontend && npm run dev"
	@echo ""
	go run main.go

clean:
	@echo "清理建立檔案..."
	rm -f dog-memory-app
	rm -rf frontend/dist
	rm -rf storage
	@echo "清理完成！"

test:
	@echo "執行測試..."
	go test -v ./...
