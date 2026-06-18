.PHONY: all build run clean

all: build

build:
	go build -o llm-webui.exe .

run: build
	./llm-webui.exe --port 42068 --db ./data.db

clean:
	rm -f llm-webui.exe llm-webui
