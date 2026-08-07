package bootstrap

import "time"

func today() string { return time.Now().Format("2006-01-02") }
