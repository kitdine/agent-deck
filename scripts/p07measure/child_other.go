//go:build !darwin

package p07measure

import "errors"

func childIdentity(int) (string, error) { return "", errors.New("P07 child lifecycle requires macOS") }
func childClock() int64                 { panic("p07: child lifecycle requires macOS") }
