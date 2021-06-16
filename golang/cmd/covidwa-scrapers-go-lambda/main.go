package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"

	csg "github.com/CovidWA/covidwa-scrapers/golang"
)

// AWS Lambda wrapper

type ScrapeEvent struct {
	Name string `json:"name"`
}

var panicError error = nil

func RunWithPanicTrap() {
	//trap any panic calls and sets the 'panicError' global variable
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				panicError = err
			} else if str, ok := r.(string); ok {
				panicError = errors.New(str)
			} else {
				panicError = fmt.Errorf("%v", r)
			}
		}
	}()

	//hard code arguments to
	args := []string{"covidwa-scrapers-go-lambda", "once"}
	csg.Run(args)
}

func HandleRequest(ctx context.Context, evt ScrapeEvent) (string, error) {
	RunWithPanicTrap()

	if panicError != nil {
		err := panicError
		panicError = nil
		return fmt.Sprintf("Execution finished with error: %s!", evt.Name), err
	} else {
		return fmt.Sprintf("Execution finished: %s!", evt.Name), nil
	}
}

func main() {
	lambda.Start(HandleRequest)
}
