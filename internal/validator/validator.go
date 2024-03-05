package validator

import (
	"regexp"
)

var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

type Validator struct {
	Errors map[string]string
}

func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

func arrContains[T comparable](x T, arr []T) bool {
	for _, v := range arr {
		if v == x {
			return true
		}
	}
	return false
}

func mapContains[T comparable](x T, arr map[T]T) bool {
	for _, v := range arr {
		if v == x {
			return true
		}
	}
	return false
}

func CheckEnvVars(envArr map[string]string, availableEnvKeys []string) bool {
	for _, key := range availableEnvKeys {
		if !mapContains(key, envArr) {
			return false
		}
	}

	for key := range envArr {
		if !arrContains(key, availableEnvKeys) {
			return false
		}
	}

	return true
}
