package util

type Issue interface {
	GetErr() error
	GetBody() string
	GetFixCallback() func() error
}

type ErrIssue struct {
	Err  error
	Desc string
}

func (e *ErrIssue) GetErr() error {
	return e.Err
}

func (e *ErrIssue) GetBody() string {
	return e.Desc
}

func (e *ErrIssue) GetFixCallback() func() error {
	return nil
}

type FileRemove struct {
	Err      error
	FileName string
	Callback func() error
}

func (f *FileRemove) GetErr() error {
	return f.Err
}

func (f *FileRemove) GetBody() string {
	return f.FileName
}

func (f *FileRemove) GetFixCallback() func() error {
	return f.Callback
}
