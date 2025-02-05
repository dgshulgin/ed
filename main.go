package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// режим работы редактора: редактирование/добавление/вставка и режим исполнения команд
type Mode int

const (
	modeAppend Mode = iota
	modeCommand
	modeQuit
)

type Handler func(*State, []string) error
type Command struct {
	name    string
	args    []string
	handler Handler
}

type State struct {
	mode Mode
	in   *bufio.Reader

	// буфер текста
	buffer []string
	// буфер изменен с момента последнего сохранения в файл
	changed bool

	// флаг отображения номеров строк
	lineNumbers bool

	// путь к открытому файлу
	filename string
}

func (state *State) quit([]string) error {
	state.mode = modeQuit
	return nil
}

// Одиночная команда . (точка) завершает режим редактирования.
func (state *State) dot([]string) error {
	state.mode = modeCommand
	return nil
}

func (state *State) append([]string) error {
	state.mode = modeAppend
	return nil
}

func (state *State) numbers([]string) error {
	state.lineNumbers = !state.lineNumbers
	return nil
}

// new очищает текстовый буфер без сохранения, создает новый документ
// TODO проверять буфер, предлагать сохранение
func (state *State) new([]string) error {
	state.buffer = nil
	return nil
}

func (state *State) print(args []string) error {
	if len(state.buffer) == 0 {
		return errors.New("text buffer is empty!")
	}
	// в аргументах гарантированно - цифры, поэтому игнорируем ошибку
	top, _ := strconv.Atoi(args[0])
	last, _ := strconv.Atoi(args[1])

	top--
	last--

	if top < 0 {
		top = 0
	}
	if last < 0 {
		last = top + 1
	} else {
		last++
	}
	if last > len(state.buffer) {
		last = len(state.buffer)
	}

	li := top
	for _, line := range state.buffer[top:last] {
		if state.lineNumbers {
			fmt.Printf("%-4d%s\n", li+1, line)
			li++
		} else {
			fmt.Printf("%s\n", line)
		}
	}
	return nil
}

func (state *State) readFile(args []string) error {
	if len(args) == 0 {
		return errors.New("File name undefined!")
	}
	fn := strings.TrimSpace(args[0])

	bb, err := readFile(fn)
	if err != nil {
		return err
	}

	state.buffer = nil
	state.buffer = append(state.buffer, bb...)
	state.filename = fn

	return nil
}

func (state *State) writeFile(args []string) error {
	if len(state.filename) == 0 && len(args) == 0 {
		return errors.New("File name undefined!\n")
	}
	var fn string
	if len(args) > 0 {
		fn = strings.TrimSpace(args[0])
	} else {
		fn = state.filename
	}

	err := writeFile(fn, state.buffer)
	if err != nil {
		return err
	}
	state.changed = false
	return nil
}

var commands map[byte]Handler = map[byte]Handler{
	'p': (*State).print,     //print buffer
	'q': (*State).quit,      //quit editor
	'a': (*State).append,    //append text
	'r': (*State).readFile,  //read file
	'w': (*State).writeFile, //write file
	'l': (*State).numbers,   //on/off line numbers
	'.': (*State).dot,
	'n': (*State).new, // новый документ
}

func (state *State) parseCommand(line []byte) (*Command, error) {
	//data := line
	if len(line) > 1 { //remove prefix .
		line = line[1:]
	}
	if peekDot(line) {
		//ret Command
		cname := line[0]
		handler, _ := commands[cname]
		return &Command{name: string(cname), args: nil, handler: handler}, nil
	}
	if peekLetter(line) {
		var top, last int = 0, len(state.buffer)

		//get command's letter
		cname := line[0]
		handler, ok := commands[cname]
		if !ok || handler == nil {
			return nil, errors.New("Command unknown!")
		}
		//fix addr
		args := make([]string, 0)
		args = append(args, fmt.Sprintf("%d", top))
		args = append(args, fmt.Sprintf("%d", last))
		//get tail
		tail := strings.TrimSpace(string(line[1:]))
		args = append(args, strings.Fields(tail)...)
		//ret Command
		return &Command{name: string(cname), args: args, handler: handler}, nil
	}
	if peekAddr(line) {
		//parse address
		var top, last int = -1, -1
		top = state.matchHere(&line)
		if line[0] == ',' {
			line = line[1:]
			last = state.matchHere(&line)

		}

		if peekLetter(line) {
			//get command's letter
			cname := line[0]
			handler, ok := commands[cname]
			if !ok || handler == nil {
				return nil, errors.New("Command unknown!")
			}
			// set up address args
			args := make([]string, 0)
			args = append(args, fmt.Sprintf("%d", top))
			args = append(args, fmt.Sprintf("%d", last))
			//get tail
			// TODO pre-calc tail's position!!
			tail := strings.TrimSpace(string(line[1:]))
			args = append(args, strings.Fields(tail)...)
			//ret Command
			return &Command{name: string(cname), args: args, handler: handler}, nil
		}
	}

	//ret Error
	return nil, errors.New("command unknown or syntax error")
}

// func (state *State) parseCommand(line []byte) (*Command, error) {

// 	data := line

// 	if len(data) > 1 { //remove prefix .
// 		data = data[1:]
// 	}
// 	cname := data[0]
// 	handler, ok := commands[cname]
// 	if !ok || handler == nil {
// 		return nil, errors.New("Command unknown!")
// 	}
// 	tail := strings.TrimSpace(string(data[1:]))
// 	args := strings.Fields(tail)
// 	cmd := Command{name: string(cname), args: args, handler: handler}
// 	return &cmd, nil
// }

func (state *State) HandleCommand(line []byte) error {
	cmd, err := state.parseCommand(line)
	if err != nil {
		return err
	}
	return cmd.handler(state, cmd.args)
}

func main() {
	state := State{
		mode:        modeCommand,
		in:          bufio.NewReader(os.Stdin),
		lineNumbers: false,
	}

	for {
		line, _, _ := state.in.ReadLine()
		if line[0] == '.' {
			err := state.HandleCommand(line)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				continue
			}
			switch state.mode {
			case modeQuit:
				fmt.Printf("Goodbye!\n")
				os.Exit(0)
				break
			}
			continue
		}
		if state.mode == modeAppend {
			state.buffer = append(state.buffer, string(line))
			state.changed = true
		}
	}
}

// peekDot Checks if the raw command line is the only '.' symbol
func peekDot(data []byte) bool {
	r, _ := utf8.DecodeRune(data)
	if r == utf8.RuneError {
		return false
	}
	if r == '.' {
		return true
	}
	return false
}

// peekLetter Checks if the raw command line starts with one of the command's list
func peekLetter(data []byte) bool {
	r, _ := utf8.DecodeRune(data)
	if r == utf8.RuneError {
		return false
	}
	return unicode.IsLetter(r)
}

// peekAddr Checks if the raw command line starts with numbers, ^ or $ and sets address or range for the [possible] command.
func peekAddr(data []byte) bool {
	r, _ := utf8.DecodeRune(data)
	if '^' == r || ',' == r || unicode.IsDigit(r) {
		return true
	}
	return false
}

// Address pattern: M-+D
// 1p			0*p
// 1,10p       0*,0*p
// ,10p		,0*p
// ^,$p		^,$p
// ^+1,$p		^+0*,$p
// ^,$-1p		^,$-0*p

// 0*
// ^+0*
// $-0*

func (state *State) matchHere(data *[]byte) int {
	var pos int

	switch (*data)[0] {
	case '^':
		pos = 1
		*data = (*data)[1:]
	case '$':
		pos = len(state.buffer)
		*data = (*data)[1:]
	default:
		pos = 0
	}

	var dir int
	switch (*data)[0] {
	case '-':
		dir = -1
		*data = (*data)[1:]
	case '+':
		dir = 1
		*data = (*data)[1:]
	default:
		dir = 1
	}

	var nn map[byte]int = map[byte]int{'1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9, '0': 0}
	var acc int = 0
	p := 0

	for {
		v, ok := nn[(*data)[p]]
		if !ok {
			break
		}
		acc = acc*10 + v
		p++
	}
	*data = (*data)[p:]

	acc *= dir
	pos += acc

	return pos
}

func readFile(filename string) ([]string, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var buffer []string
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			buffer = append(buffer, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return buffer, nil
}

func writeFile(filename string, buffer []string) error {
	file, err := os.Create(filename + ".swp")
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	for _, line := range buffer {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			file.Close()
			return err
		}
	}
	err = writer.Flush()
	if err != nil {
		file.Close()
		return err
	}
	file.Close()

	os.Remove(filename)
	os.Rename(filename+".swp", filename)

	return nil
}
