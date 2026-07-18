package service

import "errors"

var (
	ErrUnauthenticated       = errors.New("未登录或登录态丢失")
	ErrInvalidParameter      = errors.New("参数错误")
	ErrScreeningTaskNotFound = errors.New("筛选任务不存在")
)

type invalidParameterError struct {
	message string
}

func (e invalidParameterError) Error() string {
	return e.message
}

func (e invalidParameterError) Is(target error) bool {
	return target == ErrInvalidParameter
}

func newInvalidParameterError(message string) error {
	return invalidParameterError{message: message}
}
