.PHONY: build deploy

build:
	@echo "==> windres: embed icon resource into exe (taskbar icon)"
	windres icon.rc -O coff -o rsrc_windows_amd64.syso
	@echo "==> go build: mkinput.exe (windowsgui, stripped)"
	go build -ldflags "-H=windowsgui -w -s" -o build/mkinput.exe .
	@echo "==> Done: build/mkinput.exe"

# Deploy: copy build output to D:\Program Files\mkinput
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
