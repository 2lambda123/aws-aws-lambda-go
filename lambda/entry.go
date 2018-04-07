// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.

package lambda

import (
	"log"
	"net"
	"net/rpc"
	"os"
)

// Start takes a handler and talks to an internal Lambda endpoint to pass requests to the handler. If the
// handler does not match one of the supported types an appropriate error message will be returned to the caller.
// Start blocks, and does not return after being called.
//
// Rules:
//
// 	* handler must be a function
// 	* handler may take between 0 and two arguments.
// 	* if there are two arguments, the first argument must satisfy the "context.Context" interface.
// 	* handler may return between 0 and two arguments.
// 	* if there are two return values, the second argument must be an error.
// 	* if there is one return value it must be an error.
//
// Valid function signatures:
//
// 	func ()
// 	func () error
// 	func (TIn) error
// 	func () (TOut, error)
// 	func (TIn) (TOut, error)
// 	func (context.Context) error
// 	func (context.Context, TIn) error
// 	func (context.Context) (TOut, error)
// 	func (context.Context, TIn) (TOut, error)
//
// Where "TIn" and "TOut" are types compatible with the "encoding/json" standard library.
// See https://golang.org/pkg/encoding/json/#Unmarshal for how deserialization behaves
func Start(handler interface{}) {
	port := os.Getenv("_LAMBDA_SERVER_PORT")
	lis, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		log.Fatal(err)
	}
	wrappedHandler := newHandler(handler)
	function := new(Function)
	function.Handler = wrappedHandler
	err = rpc.Register(function)
	if err != nil {
		log.Fatal("failed to register handler function")
	}
	rpc.Accept(lis)
	log.Fatal("accept should not have returned")
}
