package srvDiscover

import "time"

type RegisterOption struct {
	TTLSec         int64
	Namespace      string
	BeforeRegister BeforeRegisterFunc
	ResultCallback RegisterResultCallback
	AlwaysUpdate   bool
	Interval       time.Duration
	ConnTimeout    time.Duration
}

// 注册提供的默认值
var defaultRegisterOption RegisterOption = RegisterOption{
	TTLSec:         6,
	Namespace:      DEFAULT_NAMESPACE,
	BeforeRegister: nil,
	AlwaysUpdate:   false,
	Interval:       2 * time.Second,
	ConnTimeout:    2 * time.Second,
}

type RegisterOptionFunc func(registerOp *RegisterOption)
type RegisterResultCallback func(err error)

func WithTTL(ttlSec int64) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.TTLSec = ttlSec
		if option.TTLSec <= 0 {
			option.TTLSec = defaultRegisterOption.TTLSec
		}
	}
}

func WithRegisterNamespace(namespace string) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.Namespace = namespace
		if len(option.Namespace) == 0 {
			option.Namespace = defaultRegisterOption.Namespace
		}
	}
}

func WithBeforeRegister(beforeRegister BeforeRegisterFunc) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.BeforeRegister = beforeRegister
	}
}

func WithRegisterInterval(interval time.Duration) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.Interval = interval
		if option.Interval <= 0 {
			option.Interval = defaultRegisterOption.Interval
		}
	}
}
func WithRegisterConnTimeout(timeout time.Duration) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.ConnTimeout = timeout
		if option.ConnTimeout <= 0 {
			option.ConnTimeout = defaultRegisterOption.ConnTimeout
		}
	}
}
func WithRegisterResultCallback(callback RegisterResultCallback) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.ResultCallback = callback
	}
}

type BeforeRegisterFunc func(srvInfo *RegisterInfo)

func WithRegisterAlwaysUpdate(isLoop bool) RegisterOptionFunc {
	return func(option *RegisterOption) {
		option.AlwaysUpdate = isLoop
	}
}
