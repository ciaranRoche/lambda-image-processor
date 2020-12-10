FUNCTION_NAME=DatabaseImage
QUEUE_ARN="arn:aws:sqs:eu-west-1:442832839294:imageQueue"

.PHONY: build
build:
	GOOS=linux go build main.go
	zip function.zip main

.PHONY: update
update: build
	aws lambda update-function-code --function-name  $(FUNCTION_NAME) --zip-file fileb://function.zip

.PHONY: create
create: build
	aws lambda create-function --function-name $(FUNCTION_NAME) --zip-file fileb://function.zip --handler main --runtime go1.x --timeout 60 --memory-size 1024 --role arn:aws:iam::442832839294:role/greyscale-db-create-role

.PHONY: create/queue
create/queue:
	aws lambda create-event-source-mapping --function-name DatabaseImage --batch-size 10 --event-source-arn $(QUEUE_ARN)

.PHONY: list
list:
	aws lambda list-event-source-mappings --function-name ProcessSQSRecord --event-source-arn $(QUEUE_ARN)