package fec

import "errors"

var (
	ErrTooManyMissing = errors.New("fec: มากกว่า 1 chunk หายในกลุ่มเดียว กู้คืนไม่ได้")
	ErrInvalidIndex   = errors.New("fec: missing index ไม่ถูกต้อง")
)
