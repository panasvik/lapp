package util

type Issue interface {
	GetErr() error
	GetBody() string
	GetFixCallback() func() error
}
