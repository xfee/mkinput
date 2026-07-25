.PHONY: build deploy

build:
	# -H=windowsgui 不弹出命令行黑框 -w 去掉调试信息 -s 去掉符号信息
	go build -ldflags "-H=windowsgui -w -s" -o build/mkinput.exe .

# 部署：将编译产物复制到 D:\Program Files\mkinput
deploy:
	@if [ ! -f "build/mkinput.exe" ]; then \
		echo "[ERROR] build/mkinput.exe not found, run 'make build' first"; \
		exit 1; \
	fi
	@mkdir -p "/d/Program Files/mkinput"
	@if [ -f "/d/Program Files/mkinput/mkinput.exe" ]; then \
		echo "[INFO] Replacing existing file..."; \
		rm -f "/d/Program Files/mkinput/mkinput.exe"; \
	fi
	@cp "build/mkinput.exe" "/d/Program Files/mkinput/mkinput.exe"
	@echo "[DONE] Copied to D:\Program Files\mkinput\mkinput.exe"
