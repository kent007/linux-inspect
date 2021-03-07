package top

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	now := time.Now()
	rows, iterations, err := Get(DefaultExecPath, 0, 1, 1)
	if err != nil {
		t.Skip(err)
	}
	for _, elem := range rows {
		fmt.Printf("%+v\n", elem)
	}
	fmt.Printf("found %d entries in %d iterations over %v", len(rows), iterations, time.Since(now))
}

func TestTimedGet(t *testing.T) {
	now := time.Now()
	log.Printf("starting test at %s", now.String())
	stopTimestamp := now.Add(4*time.Second + 300*time.Millisecond).UnixNano()
	output, err := GetTimed(DefaultExecPath, 0, stopTimestamp, .2)
	if err != nil {
		t.Skip(err)
	}
	rows, iterations, err := Parse(output)
	if err != nil {
		t.Skip(err)
	}
	//for _, elem := range rows {
	//	fmt.Printf("%+v\n", elem)
	//}
	log.Printf("ending test at %s", time.Now().String())
	log.Printf("found %d entries in %d iterations over %v", len(rows), iterations, time.Since(now))
}
