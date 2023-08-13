package utils

import "strings"

func Map[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func Find[T any](ss []T, test func(T) bool) (ret T, ok bool) {
	for _, s := range ss {
		if test(s) {
			ret = s
			return ret, true
		}
	}
	return ret, false
}

func Unique[T comparable](ss []T) []T {
	seen := make(map[T]bool)
	ret := make([]T, 0)
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = true
			ret = append(ret, s)
		}
	}
	return ret
}

func Contains[T comparable](ss []T, s T) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func ContainsIf[T any](ss []T, test func(T) bool) bool {
	for _, x := range ss {
		if test(x) {
			return true
		}
	}
	return false
}

func Prefix(s string, prefixes []string) (string, bool) {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return prefix, true
		}
	}
	return s, false
}
