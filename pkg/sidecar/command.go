package sidecar

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseRenderedMountCommand extracts the last simple command rendered by the
// sidecar builder and returns its argv without the leading shell exec builtin.
func ParseRenderedMountCommand(script string) ([]string, error) {
	var command string
	for _, line := range strings.Split(script, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			command = line
		}
	}
	if command == "" {
		return nil, fmt.Errorf("mount command is empty")
	}

	argv, err := splitShellWords(command)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 || argv[0] != "exec" {
		return nil, fmt.Errorf("mount command must start with exec")
	}
	argv = argv[1:]
	if len(argv) < 3 {
		return nil, fmt.Errorf("too few restart mount args")
	}
	if !strings.HasSuffix(argv[0], "mount.juicefs") && (len(argv) < 2 || argv[1] != "mount") {
		return nil, fmt.Errorf("restart args should use mount command")
	}
	return argv, nil
}

func splitShellWords(command string) ([]string, error) {
	var words []string
	for i := 0; i < len(command); {
		for i < len(command) && isASCIISpace(command[i]) {
			i++
		}
		if i == len(command) {
			break
		}

		var word strings.Builder
		started := false
		for i < len(command) && !isASCIISpace(command[i]) {
			started = true
			switch command[i] {
			case '\\':
				i++
				if i == len(command) {
					return nil, fmt.Errorf("trailing backslash in mount command")
				}
				word.WriteByte(command[i])
				i++
			case '\'':
				value, next, err := readSingleQuoted(command, i+1)
				if err != nil {
					return nil, err
				}
				word.WriteString(value)
				i = next
			case '"':
				value, next, err := readDoubleQuoted(command, i+1)
				if err != nil {
					return nil, err
				}
				word.WriteString(value)
				i = next
			case '$':
				if i+1 >= len(command) || command[i+1] != '\'' {
					return nil, fmt.Errorf("shell expansion is not supported in mount command")
				}
				value, next, err := readANSIQuoted(command, i+2)
				if err != nil {
					return nil, err
				}
				word.WriteString(value)
				i = next
			case '`':
				return nil, fmt.Errorf("command substitution is not supported in mount command")
			case '|', '&', ';', '<', '>':
				return nil, fmt.Errorf("shell operator %q is not supported in mount command", command[i])
			default:
				word.WriteByte(command[i])
				i++
			}
		}
		if started {
			words = append(words, word.String())
		}
	}
	return words, nil
}

func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

func readSingleQuoted(command string, start int) (string, int, error) {
	end := strings.IndexByte(command[start:], '\'')
	if end < 0 {
		return "", 0, fmt.Errorf("unterminated single quote in mount command")
	}
	end += start
	return command[start:end], end + 1, nil
}

func readDoubleQuoted(command string, start int) (string, int, error) {
	var value strings.Builder
	for i := start; i < len(command); i++ {
		switch command[i] {
		case '"':
			return value.String(), i + 1, nil
		case '\\':
			i++
			if i == len(command) {
				return "", 0, fmt.Errorf("trailing backslash in double quote")
			}
			if command[i] != '$' && command[i] != '`' && command[i] != '"' && command[i] != '\\' && command[i] != '\n' {
				value.WriteByte('\\')
			}
			if command[i] != '\n' {
				value.WriteByte(command[i])
			}
		case '$', '`':
			return "", 0, fmt.Errorf("shell expansion is not supported in mount command")
		default:
			value.WriteByte(command[i])
		}
	}
	return "", 0, fmt.Errorf("unterminated double quote in mount command")
}

func readANSIQuoted(command string, start int) (string, int, error) {
	var value strings.Builder
	for i := start; i < len(command); i++ {
		switch command[i] {
		case '\'':
			return value.String(), i + 1, nil
		case '\\':
			i++
			if i == len(command) {
				return "", 0, fmt.Errorf("trailing backslash in ANSI-C quote")
			}
			switch command[i] {
			case '\\', '\'', '"':
				value.WriteByte(command[i])
			case 'n':
				value.WriteByte('\n')
			case 'r':
				value.WriteByte('\r')
			case 't':
				value.WriteByte('\t')
			default:
				if command[i] >= '0' && command[i] <= '7' {
					j := i
					for j+1 < len(command) && j-i < 2 && command[j+1] >= '0' && command[j+1] <= '7' {
						j++
					}
					n, _ := strconv.ParseUint(command[i:j+1], 8, 8)
					value.WriteByte(byte(n))
					i = j
				} else {
					value.WriteByte(command[i])
				}
			}
		default:
			value.WriteByte(command[i])
		}
	}
	return "", 0, fmt.Errorf("unterminated ANSI-C quote in mount command")
}
