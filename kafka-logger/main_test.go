package main

import (
	"reflect"
	"testing"
)

func TestTopics(t *testing.T) {
	if got, want := topics(" user-api-calls, ,task-api-calls "), []string{"user-api-calls", "task-api-calls"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("topics() = %v, want %v", got, want)
	}
}
