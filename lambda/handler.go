// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.

package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-lambda-go/lambda/handlertrace"
)

type Handler interface {
	Invoke(ctx context.Context, payload []byte) ([]byte, error)
}

// lambdaHandler is the generic function type
type lambdaHandler func(context.Context, []byte) (interface{}, error)

// Invoke calls the handler, and serializes the response.
// If the underlying handler returned an error, or an error occurs during serialization, error is returned.
func (handler lambdaHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	response, err := handler(ctx, payload)
	if err != nil {
		return nil, err
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return responseBytes, nil
}

func errorHandler(e error) lambdaHandler {
	return func(ctx context.Context, event []byte) (interface{}, error) {
		return nil, e
	}
}

// handlerTakesContext returns whether the handler takes a context.Context as its first argument.
func handlerTakesContext(handler reflect.Type) (bool, error) {
	switch handler.NumIn() {
	case 0:
		return false, nil
	case 1:
		contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
		argumentType := handler.In(0)
		if argumentType.Kind() != reflect.Interface {
			return false, nil
		}
		if !contextType.Implements(argumentType) {
			return false, fmt.Errorf("handler takes an interface, but context.Context does not implement it: %q", argumentType.Name())
		}
		if argumentType.NumMethod() == 0 {
			// In this case, the signature of the handler is func (v interface{}).
			// We have two choices to call the handler.
			//
			//     ctx := context.TODO()
			//     handler(ctx)
			//
			// or
			//
			//     v, _ := json.Unmarshal(data)
			//     handler(v)
			//
			// Calling the handler succeeds in either case, but we need to choose one of them.
			// We choose the second for backward compatibility.
			return false, nil
		}
		return true, nil
	case 2:
		contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
		argumentType := handler.In(0)
		if argumentType.Kind() != reflect.Interface || !contextType.Implements(argumentType) {
			return false, fmt.Errorf("handler takes two arguments, but the first is not Context. got %s", argumentType.Kind())
		}
		return true, nil
	}
	return false, fmt.Errorf("handlers may not take more than two arguments, but handler takes %d", handler.NumIn())
}

func validateReturns(handler reflect.Type) error {
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	switch n := handler.NumOut(); {
	case n > 2:
		return fmt.Errorf("handler may not return more than two values")
	case n > 1:
		if !handler.Out(1).Implements(errorType) {
			return fmt.Errorf("handler returns two values, but the second does not implement error")
		}
	case n == 1:
		if !handler.Out(0).Implements(errorType) {
			return fmt.Errorf("handler returns a single value, but it does not implement error")
		}
	}

	return nil
}

// NewHandler creates a base lambda handler from the given handler function. The
// returned Handler performs JSON serialization and deserialization, and
// delegates to the input handler function.  The handler function parameter must
// satisfy the rules documented by Start.  If handlerFunc is not a valid
// handler, the returned Handler simply reports the validation error.
func NewHandler(handlerFunc interface{}) Handler {
	if handlerFunc == nil {
		return errorHandler(fmt.Errorf("handler is nil"))
	}
	handler := reflect.ValueOf(handlerFunc)
	handlerType := reflect.TypeOf(handlerFunc)
	if handlerType.Kind() != reflect.Func {
		return errorHandler(fmt.Errorf("handler kind %s is not %s", handlerType.Kind(), reflect.Func))
	}

	takesContext, err := handlerTakesContext(handlerType)
	if err != nil {
		return errorHandler(err)
	}

	if err := validateReturns(handlerType); err != nil {
		return errorHandler(err)
	}

	return lambdaHandler(func(ctx context.Context, payload []byte) (interface{}, error) {

		trace := handlertrace.FromContext(ctx)

		// construct arguments
		var args []reflect.Value
		if takesContext {
			args = append(args, reflect.ValueOf(ctx))
		}
		if (handlerType.NumIn() == 1 && !takesContext) || handlerType.NumIn() == 2 {
			eventType := handlerType.In(handlerType.NumIn() - 1)
			event := reflect.New(eventType)

			if err := json.Unmarshal(payload, event.Interface()); err != nil {
				return nil, err
			}
			if nil != trace.RequestEvent {
				trace.RequestEvent(ctx, event.Elem().Interface())
			}
			args = append(args, event.Elem())
		}

		response := handler.Call(args)

		// convert return values into (interface{}, error)
		var err error
		if len(response) > 0 {
			if errVal, ok := response[len(response)-1].Interface().(error); ok {
				err = errVal
			}
		}
		var val interface{}
		if len(response) > 1 {
			val = response[0].Interface()

			if nil != trace.ResponseEvent {
				trace.ResponseEvent(ctx, val)
			}
		}

		return val, err
	})
}
