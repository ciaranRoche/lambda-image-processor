FUNCTION_NAME=DatabaseEvent

.PHONY: build
build:
	GOOS=linux go build main.go
	zip function.zip main index.gohtml

.PHONY: update
update: build
	aws lambda update-function-code --function-name  $(FUNCTION_NAME) --zip-file fileb://function.zip

.PHONY: create
create: build
	aws lambda create-function --function-name $(FUNCTION_NAME) --zip-file fileb://function.zip --handler main --runtime go1.x --timeout 60 --memory-size 1024 --role arn:aws:iam::442832839294:role/greyscale-site-builder-role
