package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

type readLiner interface {
	ReadLine() (string, error)
}

type commandContext struct {
	args           []string
	stdin          readLiner
	stdout, stderr io.Writer
	pty            bool
	user           string
}

type command interface {
	execute(context commandContext) (uint32, error)
}

var commands = map[string]command{
	"sh":    cmdShell{},
	"true":  cmdTrue{},
	"false": cmdFalse{},
	"echo":  cmdEcho{},
	"cat":   cmdCat{},
	"ls":    cmdLs{},
	"touch": cmdTouch{},
	"mkdir": cmdMkdir{},
	"cd":    cmdCd{},
	"pwd":   cmdPwd{},
	"su":    cmdSu{},
}

var shellProgram = []string{"sh"}

func executeProgram(context commandContext) (uint32, error) {
	if len(context.args) == 0 {
		return 0, nil
	}
	command := commands[context.args[0]]
	if command == nil {
		_, err := fmt.Fprintf(context.stderr, "%v: command not found\n", context.args[0])
		return 127, err
	}
	return command.execute(context)
}

type cmdShell struct{}

func (cmdShell) execute(context commandContext) (uint32, error) {
	var prompt string
	if context.pty {
		switch context.user {
		case "root":
			prompt = "# "
		default:
			prompt = "$ "
		}
	}
	var lastStatus uint32
	var line string
	var err error
	for {
		_, err = fmt.Fprint(context.stdout, prompt)
		if err != nil {
			return lastStatus, err
		}
		line, err = context.stdin.ReadLine()
		if err != nil {
			return lastStatus, err
		}
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}
		if args[0] == "exit" {
			var err error
			var status = uint64(lastStatus)
			if len(args) > 1 {
				status, err = strconv.ParseUint(args[1], 10, 32)
				if err != nil {
					status = 255
				}
			}
			return uint32(status), nil
		}
		newContext := context
		newContext.args = args
		if lastStatus, err = executeProgram(newContext); err != nil {
			return lastStatus, err
		}
	}
}

type cmdTrue struct{}

func (cmdTrue) execute(context commandContext) (uint32, error) {
	_ = context
	return 0, nil
}

type cmdFalse struct{}

func (cmdFalse) execute(context commandContext) (uint32, error) {
	_ = context
	return 1, nil
}

type cmdEcho struct{}

func (cmdEcho) execute(context commandContext) (uint32, error) {
	_, err := fmt.Fprintln(context.stdout, strings.Join(context.args[1:], " "))
	return 0, err
}

type FileSystemNode struct {
	IsDir    bool
	Content  string
	Children map[string]*FileSystemNode
}

type FileSystemType struct {
	Root    *FileSystemNode
	Current *FileSystemNode
	Path    string
}

var FileSystem = FileSystemType{
	Root: &FileSystemNode{
		IsDir:    true,
		Children: make(map[string]*FileSystemNode),
	},
	Path: "/",
}

func init() {
	FileSystem.Current = FileSystem.Root
	FileSystem.Root.Children["usr.txt"] = &FileSystemNode{Content: "eberk0, cswyne, edan, aroullier, john, henk"}
	FileSystem.Root.Children["pwd.txt"] = &FileSystemNode{Content: "$2a$04$3ise9UoQ38ceyn6qUmb8neC8UyQnfNiog8ObMSPx.4KLV/vYU0XaC, $2a$04$Z2Orf4kkPuwncqrXae7L1uE5elj1Em9fhw4f8PmwS4POBAdvfzRPa, $2a$04$NkF1cDQf6CSkF83zfucmtO8.yChntXtG8HLB2zJJiZTiKIR2yHbTa, $2a$04$VFAUxOCo5hZuKjQqN6FW/.6TNoLQjFdId02Fk0pPhC0NmWiyUjwCW, $2a$04$y/dBmr4B7zWaNGpTNpjqUuZRHz9bxBaH0LwfEouan2283rBxoLWxu, $2a$04$ATK3lPdtQokdeoBJh.aOweV9h9yU6SMSQ24b7jXDZeUoHC0sMWmZS"}
	FileSystem.Root.Children["checking_account.txt"] = &FileSystemNode{Content: "null, 4936739041871256, null, 5133014750298309, 3531203913896199, 4405957561612502"}
}

type cmdPwd struct{}

func (cmdPwd) execute(context commandContext) (uint32, error) {
	_, err := fmt.Fprintln(context.stdout, FileSystem.Path)
	return 0, err
}

type cmdMkdir struct{}

func (cmdMkdir) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		_, err := fmt.Fprintln(context.stderr, "mkdir: missing operand")
		return 1, err
	}
	for _, dir := range context.args[1:] {
		parts := strings.Split(filepath.Clean(dir), "/")
		node := FileSystem.Current
		for _, part := range parts {
			if part == "" {
				continue
			}
			if _, exists := node.Children[part]; !exists {
				node.Children[part] = &FileSystemNode{IsDir: true, Children: make(map[string]*FileSystemNode)}
			}
			node = node.Children[part]
		}
	}
	return 0, nil
}

type cmdCd struct{}

func (cmdCd) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		FileSystem.Current = FileSystem.Root
		FileSystem.Path = "/"
		return 0, nil
	}
	targetPath := filepath.Clean(context.args[1])
	if targetPath == "/" {
		FileSystem.Current = FileSystem.Root
		FileSystem.Path = "/"
		return 0, nil
	}
	parts := strings.Split(targetPath, "/")
	node := FileSystem.Current
	for _, part := range parts {
		if part == ".." {
			// No parent traversal beyond root
			continue
		} else if part == "." || part == "" {
			continue
		} else {
			if child, exists := node.Children[part]; exists && child.IsDir {
				node = child
			} else {
				_, err := fmt.Fprintf(context.stderr, "cd: %s: No such file or directory\n", targetPath)
				return 1, err
			}
		}
	}
	FileSystem.Current = node
	FileSystem.Path = targetPath
	return 0, nil
}

type cmdCat struct{}

func (cmdCat) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		_, err := fmt.Fprintln(context.stderr, "cat: missing operand")
		return 1, err
	}
	for _, file := range context.args[1:] {
		if node, exists := FileSystem.Current.Children[file]; exists && !node.IsDir {
			_, err := fmt.Fprintln(context.stdout, node.Content)
			return 0, err
		} else {
			_, err := fmt.Fprintf(context.stderr, "cat: %s: No such file or directory\n", file)
			return 1, err
		}
	}
	return 0, nil
}

type cmdLs struct{}

func (cmdLs) execute(context commandContext) (uint32, error) {
	for file := range FileSystem.Current.Children {
		_, err := fmt.Fprintln(context.stdout, file)
		if err != nil {
			return 1, err
		}
	}
	return 0, nil
}

type cmdTouch struct{}

func (cmdTouch) execute(context commandContext) (uint32, error) {
	if len(context.args) < 2 {
		_, err := fmt.Fprintln(context.stderr, "usage: touch [-A [-][[hh]mm]SS] [-achm] [-r file] [-t [[CC]YY]MMDDhhmm[.SS]]\n[-d YYYY-MM-DDThh:mm:SS[.frac][tz]] file ...")
		return 1, err
	}
	for _, file := range context.args[1:] {
		FileSystem.Current.Children[file] = &FileSystemNode{Content: ""}
	}
	return 0, nil
}

type cmdSu struct{}

func (cmdSu) execute(context commandContext) (uint32, error) {
	newContext := context
	newContext.user = "root"
	if len(context.args) > 1 {
		newContext.user = context.args[1]
	}
	newContext.args = shellProgram
	return executeProgram(newContext)
}
