build:
	docker build -t elaston:latest .

shell: build
	docker run -it --rm -v ~/.aws:/root/.aws --entrypoint sh elaston:latest

run: build
	docker run -it --rm -v ~/.aws:/root/.aws elaston:latest 