package nettest

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type Type string

const Allow Type = "allow"
const Deny Type = "deny"

type task struct {
	Type Type
	Addr string
	Err  error
}

func Run(ctx context.Context, allow []string, deny []string, timeout time.Duration) (map[string][]string, error) {
	numTasks := len(allow) + len(deny)
	if numTasks == 0 {
		return nil, fmt.Errorf("no test conditions")
	}

	resultCh := make(chan task, 10)

	for _, addr := range allow {
		tt := task{
			Type: Allow,
			Addr: addr,
		}

		go doTest(ctx, tt, resultCh, timeout)
	}

	for _, addr := range deny {
		tt := task{
			Type: Deny,
			Addr: addr,
		}

		go doTest(ctx, tt, resultCh, timeout)
	}

	allowSuccess := make([]string, 0)
	allowFailed := make([]string, 0)
	denySuccess := make([]string, 0)
	denyFailed := make([]string, 0)
LOOP:
	for {
		select {
		case t := <-resultCh:
			switch t.Type {
			case Allow:
				if t.Err != nil {
					allowFailed = append(allowFailed, fmt.Sprintf("Allow %s: %s", t.Addr, t.Err))
				} else {
					allowSuccess = append(allowSuccess, fmt.Sprintf("Allow %s OK", t.Addr))
				}
			case Deny:
				if t.Err != nil {
					switch {
					case strings.Contains(t.Err.Error(), "connection refused"):
						fallthrough
					case strings.Contains(t.Err.Error(), "i/o timeout"):
						// test OK
						denySuccess = append(denySuccess, fmt.Sprintf("Deny %s: OK", t.Addr))
					default:
						// conn deny but unknown error type
						denyFailed = append(denyFailed, fmt.Sprintf("Deny %s %s", t.Addr, t.Err))
					}
				} else {
					// conn established
					denyFailed = append(denyFailed, fmt.Sprintf("Deny %s unexpected success connection", t.Addr))
				}
			}

			numTasks--
			if numTasks == 0 {
				break LOOP
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	result := map[string][]string{
		"allowSuccess": allowSuccess,
		"denySuccess": denySuccess,
		"allowFailed": allowFailed,
		"denyFailed": denyFailed,
	}

	var err error
	if len(allowFailed) > 0 || len(denyFailed) > 0 {
		err = fmt.Errorf("has failed test conditions")
	}

	return result, err
}

func doTest(ctx context.Context, t task, resultCh chan task, timeout time.Duration) {
	conn, err := net.DialTimeout("tcp", t.Addr, timeout)
	if err == nil {
		_ = conn.Close()
	}

	t.Err = err

	select {
	case resultCh <- t:
	case <-ctx.Done():
		return
	}
}
