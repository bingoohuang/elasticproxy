package model

import "net/http"

type BackupBean struct {
	Body []byte
	Req  *http.Request
}

type Backup interface {
	BackupOne(bean BackupBean)
}
