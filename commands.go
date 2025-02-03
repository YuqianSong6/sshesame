package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

type FileSystem struct {
	files    map[string]string // filename -> content
	directories map[string]bool // directory names
}

func newFileSystem() *FileSystem {
	return &FileSystem{
		files: map[string]string{
			"/etc/passwd":      "root:x:0:0:root:/root:/bin/bash",
			"/etc/shadow":      "root:$6$YTJ7FKnfsB4esnbS$5dvmYk2.GXVWhDo2TYGN7hCitD/wU9Cov.uZD8xsnseuf1r0ARX3qodIKiDsdoQA454b8IMPMOnUWDmDJVkeg1:19755:0:99999:7:::",
			"/var/log/syslog":  "Jan 1 11:30:53 Syslog: System booted",
			"/home/user/notes.txt": "Hello John, I'm e-mailing you about ...",
		},
		directories: map[string]bool{
			"/etc":      true,
			"/var/log":  true,
			"/home/user": true,
		},
	}
}

func (fs *FileSystem) readFile(filename string) (string, bool) {
	content, exists := fs.files[filename]
	return content, exists
}

func (fs *FileSystem) writeFile(filename, content string) {
	fs.files[filename] = content
}

func (fs *FileSystem) deleteFile(filename string) bool {
	if _, exists := fs.files[filename]; exists {
		delete(fs.files, filename)
		return true
	}
	return false
}

func (fs *FileSystem) createDirectory(dirname string) {
	fs.directories[dirname] = true
}

type cmdLs struct{}

func (cmdLs) execute(context commandContext) (uint32, error) {
	for dir := range fs.directories {
		fmt.Fprintln(context.stdout, dir, "[DIR]")
	}
	for file := range fs.files {
		fmt.Fprintln(context.stdout, file)
	}
	return 0, nil
}

type cmdCat struct{}

func (cmdCat) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		fmt.Fprintln(context.stderr, "Usage: cat <filename>")
		return 1, nil
	}
	content, exists := fs.readFile(context.args[1])
	if !exists {
		fmt.Fprintf(context.stderr, "cat: %s: No such file
", context.args[1])
		return 1, nil
	}
	fmt.Fprintln(context.stdout, content)
	return 0, nil
}

type cmdTouch struct{}

func (cmdTouch) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		fmt.Fprintln(context.stderr, "Usage: touch <filename>")
		return 1, nil
	}
	fs.writeFile(context.args[1], "")
	return 0, nil
}

type cmdEcho struct{}

func (cmdEcho) execute(context commandContext) (uint32, error) {
	if len(context.args) < 4 || context.args[len(context.args)-2] != ">" {
		fmt.Fprintln(context.stderr, "Usage: echo <text> > <file>")
		return 1, nil
	}
	filename := context.args[len(context.args)-1]
	content := strings.Join(context.args[1:len(context.args)-2], " ")
	fs.writeFile(filename, content)
	return 0, nil
}

type cmdRm struct{}

func (cmdRm) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		fmt.Fprintln(context.stderr, "Usage: rm <filename>")
		return 1, nil
	}
	if fs.deleteFile(context.args[1]) {
		return 0, nil
	}
	fmt.Fprintf(context.stderr, "rm: %s: No such file
", context.args[1])
	return 1, nil
}

type cmdMkdir struct{}

func (cmdMkdir) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		fmt.Fprintln(context.stderr, "Usage: mkdir <dirname>")
		return 1, nil
	}
	fs.createDirectory(context.args[1])
	return 0, nil
}

type cmdTrue struct{}

func (cmdTrue) execute(context commandContext) (uint32, error) {
	return 0, nil
}

type cmdFalse struct{}

func (cmdFalse) execute(context commandContext) (uint32, error) {
	return 1, nil
}

type cmdSu struct{}

func (cmdSu) execute(context commandContext) (uint32, error) {
	newContext := context
	newContext.user = "root"
	if len(context.args) > 1 {
		newContext.user = context.args[1]
	}
	newContext.args = []string{"sh"}
	return executeProgram(newContext)
}

type cmdShell struct{}

func (cmdShell) execute(context commandContext) (uint32, error) {
	fmt.Fprintln(context.stdout, "Shell started. Type 'exit' to leave.")
	return 0, nil
}

var fs = newFileSystem()

var commands = map[string]command{
	"ls":    cmdLs{},
	"cat":   cmdCat{},
	"touch": cmdTouch{},
	"echo":  cmdEcho{},
	"rm":    cmdRm{},
	"mkdir": cmdMkdir{},
	"true":  cmdTrue{},
	"false": cmdFalse{},
	"su":    cmdSu{},
	"sh":    cmdShell{},
}
