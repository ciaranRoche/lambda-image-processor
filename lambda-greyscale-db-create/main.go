package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"math/rand"
	"os"
	"time"
)


const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type GreyImage struct {
	ImageConverter string `json:"imageConverter"`
	SourceBucket   string `json:"sourceBucket"`
	SourceKey      string `json:"sourceKey"`
	SourceURL      string `json:"sourceURL"`
	ConvertBucket  string `json:"convertBucket"`
	ConvertKey     string `json:"convertKey"`
	ConvertURL     string `json:"convertURL"`
	ImageType      string `json:"imageType"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) {
	logger := logrus.WithFields(logrus.Fields{"action": "converter"})
	logger.Info("lambda greyscale db function called")

	// create session
	sess := session.Must(session.NewSession())
	dynoSvc := dynamodb.New(sess)
	snsSvc := sns.New(sess)


	// handle all records from sqs event
	var images []GreyImage
	for _, message := range sqsEvent.Records {
		logger.Infof("received message %s for event source %s", message.MessageId, message.EventSource)
		logger.Infof("sqs message received : %s", message.Body)

		// parse sns message, as sns topic is wrapped as sqs message
		var snsMessage sns.PublishInput
		if err := json.Unmarshal([]byte(message.Body), &snsMessage); err != nil {
			handleError(errors.Wrapf(err, "error unmarshalling sqs message"), snsSvc)
			continue
		}

		// ensure sns message is not nil
		if snsMessage.Message == nil {
			handleError(errors.New("sns message can not be nil"), snsSvc)
			continue
		}
		logger.Infof("sns message received: %s", *snsMessage.Message)

		// parse the message to type of image
		var imageMeta GreyImage
		if err := json.Unmarshal([]byte(*snsMessage.Message), &imageMeta); err != nil {
			handleError(errors.Wrapf(err, "error unmarshalling sqs message "), snsSvc)
			continue
		}
		logger.Infof("received image message : %s", imageMeta)

		// add parsed image to array of images
		images = append(images, imageMeta)
	}

	// ensure images are not nil
	if len(images) == 0 {
		handleError(errors.New("images can not be nil"), snsSvc)
		return
	}

	// handle every image
	for _, image := range images {
		// create random key for db
		image.ImageConverter = randString(10)
		// parse image as dynamodb attribute
		img, err := dynamodbattribute.MarshalMap(image)
		if err != nil {
			handleError(errors.Wrapf(err, "could not marshal image "), snsSvc)
			continue
		}

		// add image to dynamodb
		logger.Infof("adding image to dynamodb : %s", image)
		_, err = dynoSvc.PutItem(&dynamodb.PutItemInput{
			Item:      img,
			TableName: aws.String("Image"),
		})
		if err != nil {
			handleError(errors.Wrapf(err, "failure to insert item to dynamoDB"), snsSvc)
			continue
		}
	}
}

func handleError(err error, snsSvc *sns.SNS) {
	log := logrus.WithFields(logrus.Fields{"action": "error"})
	log.Error(err)

	// publish error message to sns topic
	_, err = snsSvc.Publish(&sns.PublishInput{
		Message: aws.String(fmt.Sprintf("error : %v", err)),
		TopicArn: aws.String(os.Getenv("ERRORSNS")),
	})
	if err != nil {
		// fail gracefully
		log.Errorf("error publishing sns : %v", err)
	}
}

func main() {
	lambda.Start(handler)
}

// build random string helper func
func stringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// build random string wrapper func
func randString(length int) string {
	return stringWithCharset(length, charset)
}