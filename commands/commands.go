// Package commands registers and executes commands (and their Close functions)
package commands

type GetQueryValFunc func(varName string) (string, bool)
type CommandFunc func(getQueryVal GetQueryValFunc) string

var items = make(map[string]CommandFunc)
var closeFuncs = make(map[string]func())

func Add(name string, val CommandFunc) {
	items[name] = val
}
func Get(name string) (CommandFunc, bool) {
	val, ok := items[name]
	return val, ok
}

func AddCloseFunc(name string, theFunc func()) {
	closeFuncs[name] = theFunc
}
func RunCloseFuncs() {
	for _, closeFunc := range closeFuncs {
		closeFunc()
	}
}
