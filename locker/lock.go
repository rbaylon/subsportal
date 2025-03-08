package locker

import "log"

var Lock bool

func GetLock(b *bool, caller string) bool {
	log.Printf("(%s) Returned lock: %v\n", caller, *b)
	return *b
}

func SetLock(b *bool, val bool, caller string) {
	log.Printf("(%s) Lock set to: %v\n", caller, val)
	*b = val
}
