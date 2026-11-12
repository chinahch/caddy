//go:build !linux

package agent

import "errors"

func LockState(dir string) (func(), error) {
	return nil, errors.New("worker state locking requires Linux")
}
