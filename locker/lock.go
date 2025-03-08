package locker

var lock bool = false

func GetLock() *bool {
	return &lock
}
