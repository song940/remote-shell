package terminal

import (
	"errors"
	"fmt"
)

var ErrFlagNotSet = errors.New("Flag not set")

type Node interface {
	Value() string
	Start() int
	End() int
	Type() string
}

type baseNode struct {
	start, end int
	value      string
}

func (bn *baseNode) Value() string {
	return bn.value
}

func (bn *baseNode) Start() int {
	return bn.start
}

func (bn *baseNode) End() int {
	return bn.end
}

type Argument struct {
	baseNode
}

func (a Argument) Type() string {
	return "argument"
}

type Cmd struct {
	baseNode
}

func (c Cmd) Type() string {
	return "command"
}

type Flag struct {
	baseNode

	Args []Argument
	long bool
}

func (f Flag) Type() string {
	return "flag"
}

func (f *Flag) ArgValues() (out []string) {
	for _, v := range f.Args {
		out = append(out, v.Value())
	}
	return
}

type ParsedLine struct {
	FlagsOrdered []Flag
	Flags        map[string]Flag

	Arguments []Argument
	Focus     Node

	Section *Flag

	Command *Cmd

	RawLine string
}

func (pl *ParsedLine) ArgumentsAsStrings() (out []string) {
	for _, v := range pl.Arguments {
		out = append(out, v.Value())
	}
	return
}

func (pl *ParsedLine) IsSet(flag string) bool {
	_, ok := pl.Flags[flag]
	return ok
}

func (pl *ParsedLine) ExpectArgs(flag string, needs int) ([]Argument, error) {
	f, ok := pl.Flags[flag]
	if ok {
		if len(f.Args) != needs {
			return nil, fmt.Errorf("flag: %s expects %d arguments", flag, needs)
		}
		return f.Args, nil
	}
	return nil, ErrFlagNotSet
}

func (pl *ParsedLine) GetArgs(flag string) ([]Argument, error) {
	f, ok := pl.Flags[flag]
	if ok {
		return f.Args, nil
	}
	return nil, ErrFlagNotSet
}

func (pl *ParsedLine) GetArgsString(flag string) ([]string, error) {
	f, ok := pl.Flags[flag]
	if ok {
		return f.ArgValues(), nil
	}
	return nil, ErrFlagNotSet
}

func (pl *ParsedLine) GetArg(flag string) (Argument, error) {
	arg, err := pl.ExpectArgs(flag, 1)
	if err != nil {
		return Argument{}, err
	}

	return arg[0], nil
}

func (pl *ParsedLine) GetArgString(flag string) (string, error) {
	f, ok := pl.Flags[flag]
	if !ok {
		return "", ErrFlagNotSet
	}

	if len(f.Args) == 0 {
		return "", fmt.Errorf("flag: %s expects at least 1 argument", flag)
	}
	return f.Args[0].Value(), nil

}

func parseFlag(line string, startPos int) (f Flag, endPos int) {

	f.start = startPos
	linked := true
	for f.end = startPos; f.end < len(line); f.end++ {
		endPos = f.end
		if line[f.end] == ' ' {

			return
		}

		if line[f.end] == '-' && linked {
			continue
		}

		if f.end-startPos > 1 && linked {
			f.long = true
		}

		linked = false

		f.value += string(line[f.end])
	}

	return
}

func parseSingleArg(line string, startPos int) (arg Argument, endPos int) {
	arg.start = startPos

	for arg.end = startPos; arg.end < len(line); arg.end++ {
		endPos = arg.end

		if line[endPos] == ' ' {
			return
		}

		arg.end = endPos
		arg.value += string(line[endPos])
	}

	return
}

func parseArgs(line string, startPos int) (args []Argument, endPos int) {

	for endPos = startPos; endPos < len(line); endPos++ {

		var arg Argument
		arg, endPos = parseSingleArg(line, endPos)

		if len(arg.value) != 0 {
			args = append(args, arg)
		}

		if endPos != len(line)-1 && line[endPos+1] == '-' {
			return
		}
	}

	return
}

func ParseLine(line string, cursorPosition int) (pl ParsedLine) {

	var capture *Flag = nil
	pl.Flags = make(map[string]Flag)
	pl.RawLine = line

	for i := 0; i < len(line); i++ {
		if i < len(line) && line[i] == '-' {

			if capture != nil {
				pl.Flags[capture.Value()] = *capture
				pl.FlagsOrdered = append(pl.FlagsOrdered, *capture)
			}

			var newFlag Flag
			newFlag, i = parseFlag(line, i)
			if cursorPosition >= newFlag.start && cursorPosition <= newFlag.end {
				pl.Focus = &newFlag
				pl.Section = &newFlag
			}

			//First start parsing long options --blah
			if newFlag.long {
				capture = &newFlag
				continue
			}

			//Start short option parsing -l or -ltab = -l -t -a -b

			//For a single option, its not ambigous for what option we're capturing an arg for
			if len(newFlag.Value()) == 1 {
				capture = &newFlag
				continue
			}

			//Most of the time its ambigous with multiple options in one flag, e.g -aft what arg goes with what option?
			capture = nil
			for _, c := range newFlag.Value() {
				//Due to embedded struct this has to be like this
				var f Flag
				f.start = newFlag.start
				f.end = i
				f.value = string(c)

				pl.Flags[f.Value()] = f
				pl.FlagsOrdered = append(pl.FlagsOrdered, f)
			}
			continue

		}

		var args []Argument
		args, i = parseArgs(line, i)
		pl.Arguments = append(pl.Arguments, args...)

		for m, arg := range args {
			if cursorPosition >= arg.start && cursorPosition <= arg.end {
				pl.Focus = &args[m]

				pl.Section = capture
			}
		}

		if capture != nil {
			capture.Args = args
			continue
		}

	}

	if capture != nil {
		pl.Flags[capture.Value()] = *capture
		pl.FlagsOrdered = append(pl.FlagsOrdered, *capture)
	}

	var closestLeft *Flag

	for i := len(pl.FlagsOrdered) - 1; i >= 0; i-- {
		if cursorPosition >= pl.FlagsOrdered[i].start && cursorPosition <= pl.FlagsOrdered[i].end {
			pl.Section = &pl.FlagsOrdered[i]
			break
		}

		if pl.FlagsOrdered[i].end > cursorPosition {
			continue
		}

		closestLeft = &pl.FlagsOrdered[i]
		break
	}

	if pl.Section == nil && closestLeft != nil {
		pl.Section = closestLeft
	}

	if pl.Command == nil && len(pl.Arguments) > 0 {
		pl.Command = new(Cmd)
		pl.Command.value = pl.Arguments[0].value
		pl.Command.start = pl.Arguments[0].start
		pl.Command.end = pl.Arguments[0].end

		if cursorPosition >= pl.Command.start && cursorPosition <= pl.Command.end {
			pl.Focus = pl.Command
		}

		pl.Arguments = pl.Arguments[1:]
	}

	return

}

func absInt(x int) int {
	if x < 0 {
		return 0 - x
	}
	return x - 0
}
