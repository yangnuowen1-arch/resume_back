package service

import "errors"

var ErrUnauthenticated = errors.New("未登录或登录态丢失")
