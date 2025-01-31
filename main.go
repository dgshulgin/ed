package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

func (state *State) print([]string) error {
	if len(state.buffer) == 0 {
		return errors.New("text buffer is empty!")
	}

	for li, line := range state.buffer {
		if state.lineNumbers {
			fmt.Printf("%-4d%s\n", li+1, line)
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

	data := line

	if len(data) > 1 { //remove prefix .
		data = data[1:]
	}
	cname := data[0]
	handler, ok := commands[cname]
	if !ok || handler == nil {
		return nil, errors.New("Command unknown!")
	}
	tail := strings.TrimSpace(string(data[1:]))
	args := strings.Fields(tail)
	cmd := Command{name: string(cname), args: args, handler: handler}
	return &cmd, nil
}

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
