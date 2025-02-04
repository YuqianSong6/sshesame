package main

import (
	"fmt"
	"io"
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

var FileSystem = map[string]string{
	"usr.txt": "eberk0, cswyne, edan, aroullier, john, henk",
	"pwd.txt": "$2a$04$3ise9UoQ38ceyn6qUmb8neC8UyQnfNiog8ObMSPx.4KLV/vYU0XaC, $2a$04$Z2Orf4kkPuwncqrXae7L1uE5elj1Em9fhw4f8PmwS4POBAdvfzRPa, $2a$04$NkF1cDQf6CSkF83zfucmtO8.yChntXtG8HLB2zJJiZTiKIR2yHbTa, $2a$04$VFAUxOCo5hZuKjQqN6FW/.6TNoLQjFdId02Fk0pPhC0NmWiyUjwCW, $2a$04$y/dBmr4B7zWaNGpTNpjqUuZRHz9bxBaH0LwfEouan2283rBxoLWxu, $2a$04$ATK3lPdtQokdeoBJh.aOweV9h9yU6SMSQ24b7jXDZeUoHC0sMWmZS",
	"cc.txt":  "null, 4936739041871256, null, 5133014750298309, 3531203913896199, 4405957561612502",
}

type cmdCat struct{}

func (cmdCat) execute(context commandContext) (uint32, error) {
	if len(context.args) > 1 {
		for _, file := range context.args[1:] {
			if content, exists := FileSystem[file]; exists {
				_, err := fmt.Fprintln(context.stdout, content)
				if err != nil {
					return 0, err
				}
			} else {
				_, err := fmt.Fprintf(context.stderr, "%v: %v: No such file or directory\n", context.args[0], file)
				if err != nil {
					return 0, err
				}

			}
		}
		return 1, nil
	}
	//var line string
	//var err error
	//for err == nil {
	//	line, err = context.stdin.ReadLine()
	//	if err == nil {
	//		_, err = fmt.Fprintln(context.stdout, line)
	//	}
	//}
	return 0, nil
}

type cmdLs struct{}

func (cmdLs) execute(context commandContext) (uint32, error) {
	for file := range FileSystem {
		_, err := fmt.Fprintln(context.stdout, file)
		if err != nil {
			return 1, err
		}
	}
	return 0, nil
}

type cmdTouch struct{}

func (cmdTouch) execute(context commandContext) (uint32, error) {
	if len(context.args) > 1 {
		file := context.args[1]
		if _, exists := FileSystem[file]; exists && context.user != "root" {
			_, err := fmt.Fprintln(context.stdout, "touch: cannot touch \""+file+"\" : Permission denied")
			return 1, err
		} else {
			if len(context.args) == 2 {
				FileSystem[file] = ""
			}
		}
	} else {
		_, err := fmt.Fprintln(context.stdout, "usage: touch [-A [-][[hh]mm]SS] [-achm] [-r file] [-t [[CC]YY]MMDDhhmm[.SS]]\n[-d YYYY-MM-DDThh:mm:SS[.frac][tz]] file ...")
		return 1, err
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
